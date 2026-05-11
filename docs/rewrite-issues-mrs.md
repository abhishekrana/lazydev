# lazydev rewrite — focus on Issues + MRs, SQLite cache, AI workflow

## Context

`lazydev` is currently a unified TUI for Docker / K8s / Logs / GitLab. The scope has drifted from how the tool is actually used: the daily-driver value is GitLab Issues + MRs, especially for triaging work between the human user and AI agents (Claude Code) acting as bot users. The Docker/K8s/Logs surfaces add ~1,700 LOC of code, dependencies (Docker SDK, client-go), and maintenance load with no current payoff.

Three problems to fix in this rewrite:

1. **Scope bloat** — delete everything except Issues + MRs (the GitLab client itself is mostly kept, minus pipelines).
2. **Latency on every interaction** — today every tab does a fresh API call on Init, on every 30s tick, and on detail open (3 calls per issue, 2 per MR). Nothing is cached. Sidebar takes 1–3 seconds to populate on every cold start and on every refresh tick.
3. **No offline mode** — pull the network and the app is useless. The user wants a local mirror of the last ~30 days of project activity so they can browse, search, and triage offline; when the network returns, the cache catches up automatically.

Goal: a focused, snappy, AI-workflow-aware TUI where the sidebar is instant from disk, the last 30 days of activity is browseable offline, background sync keeps things fresh, and the user can hand tickets/MRs between themselves and a Claude bot account fluently.

**Primary purpose**: lazydev is the cockpit for pair-programming with Claude on GitLab projects. The shortest possible loop is: _query fast → multi-select context → one keypress packs it as a Claude-ready prompt → paste into `claude -p` (or pipe directly) → review the resulting MR back in lazydev → iterate_. Every feature in v1 is evaluated against "does this shorten that loop?" — features that don't earn their keep are out of scope.

## Decisions (from clarifying questions)

- **Branch**: `rewrite-issues-mrs` off `main` (currently at `c0fc9a0`).
- **Cache**: SQLite only (no separate in-memory layer) via `modernc.org/sqlite` (pure Go, no CGO) at `~/.local/state/lazydev/cache.db`. At this scale (<10k rows expected) SQLite reads are sub-millisecond — a Go-side cache adds invalidation complexity for no perceptible speed gain. Tabs naturally hold the current view slice as struct state (so Bubble Tea redraws don't re-query on every frame) — that's UI state, not a cache.
- **Sync model**: background sync every ~20s using `updated_after=MAX(updated_at)`; sidebar renders from SQLite on startup with zero network wait.
- **Prefetch window**: on first run (or whenever the cache is empty), backfill the last 30 days of _all_ issues and MRs in the configured project — not just items assigned to the tracked users. This gives a true offline mode and powers search across tickets the user didn't author. Window is configurable (`cache.prefetch_window_days`, default 30).
- **Retention**: closed/merged items older than the prefetch window are pruned by a background janitor. Open items are kept forever regardless of age — they're still actionable.
- **Layout**: keep separate Issues and MRs tabs.
- **Query DSL on `/`**: k9s/GitLab-style — `key:value` operators (`assignee:@me`, `assignee:claude-bot`, `author:<name>`, `label:bug`, `state:open|closed|merged|all`, `milestone:%current`, `updated:>7d`) AND'd together, plus bare terms as fuzzy match across title+description+notes via FTS5. Single-line, live-applied. Works in both Issues and MRs tabs and as a global search overlay.
- **Saved views**: `:save <name> <expr>` writes a named query to `~/.config/lazydev/views.yaml`. Number keys `1`–`9` recall views directly. Hardcoded grouping logic ("My Issues / Other Issues / All Issues") is replaced by user-defined views.
- **Multi-select + context export**: `Space` toggles mark on the current row (`v`/`V` for visual range); `y` copies the marked items to clipboard via OSC52; `Y` writes them to `/tmp/lazydev-ctx-<ts>.md`; `Ctrl+Enter` pipes them to a configurable `llm_command` (default `claude -p`). Output format defaults to Claude XML for multi-item, Markdown for single-item. `Shift+Y` does a "deep pack" that inlines related-MR bodies + diff stats.
- **AI workflow keybindings**: `N` quick-create issue → auto-assign to `ai_user`; `T` toggle assignee self ↔ `ai_user`; AI Queue is a saved view, not a hardcoded group.
- **No redaction in v1** (deferred): export passes content as-is. The user is responsible for not putting secrets in tickets. Revisit if/when a leak happens.
- **No streaming/automation in v1** (deferred): lazydev produces the prompt; the user runs Claude in another pane. See "Future work" section below for Tier 2/3 ideas.
- **Scope cuts**: delete Pipelines, Docker, Kubernetes, Dashboard, Logs tabs and their backing packages.

## Step 1 — Cut scope

Create branch and delete in one commit:

- `git checkout -b rewrite-issues-mrs` (done).
- Delete tab files: `internal/ui/tabs/{pipelines,docker,kubernetes,dashboard,logs}.go` (~1,738 LOC).
- Delete packages: `internal/docker/`, `internal/kube/`, `internal/log/`, `internal/discovery/`. **Keep `internal/export/`** — its `ToClipboardOSC52` and `LinesToText`/`LinesToJSON` helpers are reused for the context-export feature. Prune unused functions from it; do not delete the package.
- Delete GitLab pipeline code: `internal/gitlab/pipelines.go` (-240 LOC), and remove `GitLabPipeline`, `GitLabJob`, `PipelineListMsg`, pipeline-related messages from `pkg/messages/messages.go`.
- Prune `internal/app/app.go` `SharedState`: drop `DockerClient`, `KubeClient`, `StreamManager`. Keep `GitLabClient`, `Config`, and add `Cache *cache.Store` and `Syncer *cache.Syncer` (new).
- Prune `cmd/lazydev/main.go`: remove Docker/K8s init + discovery, register only Issues + MRs tabs.
- Prune `internal/ui/root.go`: drop tab IDs 1–5 (Docker/K8s/Logs/Dashboard/Pipelines), keep Issues + MRs. Update tab key bindings (`1` = Issues, `2` = MRs).
- Update `internal/config/config.go`: remove Docker, Kubernetes, Logs config sections. Keep `GitLab` and add `Cache` section (`db_path`, `sync_interval_s`, `prefetch_window_days`, `ai_user`).
- `go mod tidy` to drop unused deps (`docker/docker`, `client-go`, etc.).
- `task check` must pass.

## Step 2 — Cache layer (`internal/cache/`)

New package. Files:

- `cache/store.go` — owns `*sql.DB`, opens/migrates on startup. Public surface:
  - `Open(path string) (*Store, error)` — opens DB, runs migrations.
  - `Close() error`.
  - `UpsertIssues(items []messages.GitLabIssue) error` — single transaction.
  - `UpsertMRs(items []messages.GitLabMR) error`.
  - `UpsertNotes(parentType string, parentIID int, notes []messages.GitLabNote) error`.
  - `ListIssues(filter Filter) ([]messages.GitLabIssue, error)` — replaces the current `ListMyIssues()` for reads.
  - `ListMRs(filter Filter) ([]messages.GitLabMR, error)`.
  - `GetIssue(iid int) (*messages.GitLabIssue, []messages.GitLabNote, error)`.
  - `GetMR(iid int) (*messages.GitLabMR, []messages.GitLabNote, error)`.
  - `MaxIssueUpdatedAt() (time.Time, error)` / `MaxMRUpdatedAt() (time.Time, error)` — drives `updated_after` sync.
  - `Search(query string, limit int) ([]SearchHit, error)` — FTS5 query, ranked.

- `cache/schema.go` — embedded SQL migrations. Tables:
  - `issues(iid INTEGER PRIMARY KEY, project_id, title, description, state, author_username, assignee_username, labels TEXT, milestone TEXT, iteration_title TEXT, iteration_start, iteration_due, web_url, created_at, updated_at, raw_json BLOB)`
  - `mrs(iid INTEGER PRIMARY KEY, ...)` — analogous, plus `source_branch`, `target_branch`, `merge_status`, `approved INT`.
  - `notes(id INTEGER PRIMARY KEY, parent_type TEXT, parent_iid INT, body, author, created_at)` — indexed on `(parent_type, parent_iid)`.
  - `related_mrs(issue_iid INT, mr_iid INT, PRIMARY KEY(issue_iid, mr_iid))`.
  - `meta(key TEXT PRIMARY KEY, value TEXT)` — `last_full_sync`, `schema_version`.
  - `search_fts` — FTS5 virtual table over `(kind, iid, title, body, notes)`; populated by triggers from `issues`, `mrs`, `notes`.

- `cache/sync.go` — background sync loop:
  - `Syncer` struct holds `*Store`, `*gitlab.Client`, ticker, `tea.Program` handle (or output channel), `prefetchWindow time.Duration`.
  - `Start(ctx)` — goroutine logic:
    1. **First-run prefetch** (if `meta.last_full_sync` is empty or older than the prefetch window): paginate through `/issues?scope=all&updated_after=now-30d&order_by=updated_at&sort=asc` and `/merge_requests?scope=all&updated_after=now-30d` until exhausted. State filter is _not_ applied — both open and closed/merged items are stored. Done in batches of 100 with progress emitted as `SyncStatusMsg{State: "prefetching", Progress: "n/total"}`.
    2. **Incremental tick** every `cache.sync_interval_s` (default 20s): `updated_after = MAX(updated_at) - 60s` (overlap to avoid race), fetch only what's changed, upsert, emit `CacheUpdatedMsg{Kind}`.
    3. **Daily janitor**: every 24h, delete from `issues`/`mrs` where `state IN ('closed','merged') AND updated_at < now - prefetch_window`. Open items are kept forever.
  - Survives transient API errors with exponential backoff capped at 2 min; status surfaced via `SyncStatusMsg`. On network failure, the existing cache continues to serve reads (offline mode); the status bar shows `offline · last sync <time>` and the last successful sync time.
  - `SyncNow()` method on Syncer triggers an immediate incremental tick (bound to manual `r` refresh).

- `internal/gitlab/issues.go` and `internal/gitlab/mergerequests.go` get two new functions:
  - `ListIssuesUpdatedAfter(t time.Time, scope string)` — `scope="all"` fetches everything in the project, `scope="assigned_to_me"` keeps the old narrow filter. Used by Syncer for both first-run prefetch and incremental syncs.
  - `ListMRsUpdatedAfter(t time.Time, scope string)` — analogous.
  - Both paginate fully (loop until response is shorter than PerPage).
  - The existing `ListMyIssues` / `ListMyMRs` are deleted — tabs read exclusively from the cache, which already includes all "my" items because the prefetch sweep is project-wide.

- `pkg/messages/messages.go` gains: `CacheUpdatedMsg{Kind string}`, `SyncStatusMsg{State string, Progress string, LastSyncAt time.Time, Err error}` (where `State` is one of `prefetching` / `syncing` / `idle` / `offline`), `SearchHit{Kind string, IID int, Title string, Score float64}`, `SearchResultMsg{Query string, Hits []SearchHit}`.

## Step 3 — Wire tabs to the cache

`internal/ui/tabs/issues.go` and `mergerequests.go`:

- Replace direct `client.ListMyIssues()` / `ListMyMRs()` calls with `state.Cache.ListIssues(...)` / `ListMRs(...)`. This is synchronous, returns instantly from SQLite — Init no longer waits on the network.
- Replace `client.GetIssue(iid)` / `GetMR(iid)` (used after the 150ms debounce in detail-open) with: first read `state.Cache.GetIssue(iid)` synchronously and paint the detail pane immediately; then fire an async refresh (call the live API, upsert, emit `CacheUpdatedMsg`) only if `cached.UpdatedAt` is older than 60s.
- Remove the 30s `refreshS` ticker entirely — refresh is now driven by `CacheUpdatedMsg` from the Syncer. Manual `r` triggers a one-off `Syncer.SyncNow()`.
- The existing `needsAutoSelect` / `pendingFetch` / `fetchSeq` flow is preserved — only the data source changes.
- Write actions (close/reopen/comment/assign/approve/merge) still hit the live API immediately, then optimistically upsert the response into the cache so the UI updates without waiting for the next sync tick.

## Step 4 — Query DSL on `/` and saved views

Replaces the old hardcoded "My / Other / All" grouping. One input box drives the sidebar.

- `internal/query/parser.go` (new package): parses a query string into a `Query` struct. Supported tokens:
  - `key:value` — `state:open|closed|merged|all` (default `open`), `assignee:@me|<username>|@none|@any`, `author:<username>`, `label:<name>` (repeatable, AND), `milestone:%current|%<title>|@none`, `iteration:%current`, `updated:>7d|<30d|=2026-05-01`, `kind:issue|mr|both`.
  - `@me` resolves to the authenticated user; `@ai` resolves to `cfg.GitLab.AIUser`.
  - Bare terms — fuzzy match across title+description+notes via FTS5.
  - Whitespace separates terms, all AND'd; `|` between two terms = OR within the same field.
- `internal/cache/store.go` gains `Query(q Query) ([]messages.GitLabIssue, []messages.GitLabMR, error)` — translates `Query` to a single SQL statement against `issues` + `mrs` + `search_fts`. Returns whichever kind(s) the query asks for.
- `internal/ui/components/queryline.go` (new): single-line input shown above the sidebar, bound to `/`. Live-applied on every keystroke; debounced 30ms. Esc closes and clears. Shows match count in the right edge.
- **Saved views** (`internal/ui/views/views.go` + `~/.config/lazydev/views.yaml`):
  - `:save <name> <expr>` writes `{name, expr}` to the YAML file.
  - `:del <name>` removes one.
  - `1`–`9` recall the views in declaration order; `gv<n>` for views beyond 9.
  - Default views shipped on first run (written to `views.yaml` if missing): `1=mine` (`assignee:@me state:open`), `2=ai-queue` (`assignee:@ai state:open`), `3=review` (just MRs awaiting your review), `4=recent` (`updated:<7d`).
  - The Issues and MRs tab titles in the tab bar show the active view name; switching views never switches tabs (a view can be `kind:both`).
- The previous `AI Queue` group concept is dropped — it's just a saved view now.

## Step 5 — Multi-select and Claude context export

This is the keystroke that earns lazydev its keep.

- `internal/ui/components/sidebar.go` — add multi-select state: `markedIIDs map[int]struct{}`. Bind `Space` to toggle the cursor item. `v` starts visual range mode (cursor moves extend the range until `Esc`); `V` is line-mode equivalent for whole groups. Status bar shows `N selected`.
- `internal/export/context.go` (new file in the kept `export` package): builds the export payload.
  - `BuildClaudeXML(items []ExportItem) string` — wraps each item in:
    ```xml
    <issue id="#123" url="..." state="open" labels="bug,backend" assignee="alice" updated="2026-05-08">
      <title>...</title>
      <body>...</body>
      <comments>
        <comment author="bob" at="2026-05-08T10:00">...</comment>
      </comments>
      <related_mrs>
        <mr id="!567" state="merged" url="...">Add token refresh</mr>
      </related_mrs>
    </issue>
    ```
    Per Anthropic's prompt-engineering guidance: XML tags for multi-document context. MR items use `<mr>` with `<source_branch>`, `<target_branch>`, `<diff_stat>`.
  - `BuildMarkdown(items []ExportItem) string` — `## ISSUE-123: title` heading format for single items or when `format=markdown` in config.
  - `BuildDeep(items []ExportItem) string` — Shift+Y; inlines related-MR bodies + diff stats (fetched fresh from GitLab API, not the cache, to get the diff).
- Keybindings (in both Issues and MRs tabs, active when any items are selected):
  - `y` — copy XML/Markdown to clipboard via OSC52. Status bar: `copied N items to clipboard`.
  - `Y` — write to `/tmp/lazydev-ctx-<ISO8601>.md`. Status bar shows the path.
  - `Ctrl+Enter` — pipe to `cfg.Export.LLMCommand` (default `claude -p`); exec via `os/exec` with the payload on stdin, in the user's `$SHELL`. Streams output to a notification-bar tail and exits when the process exits. If `tty` is needed, fall back to detaching the process and writing the payload to a tempfile that the command reads (configurable via `{{.File}}` template in `llm_command`).
  - `Shift+Y` — same as `Y` but uses `BuildDeep`. Notifies on completion since it makes extra API calls.
  - `Esc` clears selection.
- Config additions (`internal/config/config.go`, new `Export` section):
  ```yaml
  export:
    format: claude-xml # or "markdown"
    llm_command: "claude -p" # or "aider --message-stdin" etc.
    include_comments: true
    include_related_mrs: stub # "stub" | "full" | "none"
  ```

## Step 6 — AI-handoff keybindings on the selected item

Small additions, on top of the multi-select export above.

- **`N` — Quick-create issue, assigned to AI**: opens `$EDITOR` with `# Title\n\n## Description\n`, parses on save, calls `gitlab.CreateIssue(title, body)` then `gitlab.AssignIssue(newIID, aiUserID)`. Reuses the editor plumbing already used by `c` (comment). Upserts into cache so the new issue appears in the AI-queue saved view within milliseconds.
- **`T` — Toggle assignee self ↔ AI**: on the cursor item, flip `assignee_username` between current user and `cfg.GitLab.AIUser`. Optimistic cache upsert. Reuses `gitlab.AssignIssue` (or MR equivalent).
- **`a` — Assign self** (already exists, kept): unchanged.

## Step 7 — Tabs read from cache (no behavior change beyond data source)

`internal/ui/tabs/issues.go` and `mergerequests.go`:

- Sidebar items come from `state.Cache.Query(activeView.Query)`. The active view's expression is mutated by `/` (transient query) and `1`–`9` (saved view recall).
- Detail open reads `state.Cache.GetIssue(iid)` synchronously and paints immediately. If the cached `UpdatedAt` is older than 60s, fire an async refresh (live API → upsert → emit `CacheUpdatedMsg`).
- The 30s `refreshS` ticker is removed — redraws are driven by `CacheUpdatedMsg` from the Syncer. Manual `r` calls `Syncer.SyncNow()`.
- Existing `needsAutoSelect` / `pendingFetch` / `fetchSeq` flow is preserved.
- Write actions (close/reopen/comment/assign/approve/merge) still hit the live API and optimistically upsert the response into the cache, so the UI updates without waiting for the next sync tick.

## Critical files to modify

- `cmd/lazydev/main.go` — open cache, start syncer, register fewer tabs, load saved views.
- `internal/app/app.go` — slim `SharedState` to `{GitLabClient, Cache, Syncer, Views, Config}`.
- `internal/config/config.go` — drop Docker/K8s/Logs, add `cache`, `ai_user`, `export` sections.
- `internal/ui/root.go` — register 2 tabs, bind `/` to query line, `1`–`9` to saved views, route `CacheUpdatedMsg` and selection events.
- `internal/ui/tabs/issues.go` — drive sidebar from `Cache.Query(activeView)`, add `N` / `T` and selection-aware `y`/`Y`/`Ctrl+Enter`/`Shift+Y`, drop the 30s ticker.
- `internal/ui/tabs/mergerequests.go` — same.
- `internal/ui/components/sidebar.go` — add multi-select state (`Space`, `v`/`V`, `Esc`); render label chips next to titles when present.
- `internal/ui/components/queryline.go` — new, single-line input above the sidebar.
- `internal/gitlab/issues.go` — add `ListIssuesUpdatedAfter`.
- `internal/gitlab/mergerequests.go` — add `ListMRsUpdatedAfter` + a `GetMRDiff(iid)` helper for `Shift+Y` deep pack.
- `internal/cache/{store.go,schema.go,sync.go}` — new.
- `internal/query/parser.go` — new, parses the DSL.
- `internal/ui/views/views.go` — new, loads/saves `~/.config/lazydev/views.yaml`.
- `internal/export/context.go` — new, `BuildClaudeXML` / `BuildMarkdown` / `BuildDeep`.
- `internal/export/export.go` — keep, prune unused log-export helpers, keep OSC52 + tempfile.
- `pkg/messages/messages.go` — add `CacheUpdatedMsg`, `SyncStatusMsg`, `SelectionChangedMsg`, `ExportDoneMsg`; remove pipeline/docker/k8s messages.

## Files / packages to delete

- `internal/ui/tabs/{pipelines,docker,kubernetes,dashboard,logs}.go`
- `internal/{docker,kube,log,discovery}/` (entire packages). **Keep `internal/export/`** — OSC52 and tempfile helpers are reused by the context-export feature.
- `internal/gitlab/pipelines.go`
- Pipeline/Docker/K8s message types in `pkg/messages/messages.go`
- README sections referencing removed tabs (update at end of rewrite).

## Reused existing utilities

- `internal/ui/components/sidebar.go` — keep, extend with multi-select for export.
- `internal/ui/components/{detailpane,modal,inputmodal}.go` — keep, used by both tabs.
- `internal/gitlab/{client.go,issues.go,mergerequests.go}` — keep, extend with `*UpdatedAfter` variants. The auth discovery in `client.go` (config → env → glab CLI) is solid and unchanged.
- Markdown rendering via `glamour` in `FormatIssueDetail` / `FormatMRDetail` — keep; consider memoizing by `(iid, updated_at)` to skip re-render on redraw.
- The `additional_users` plumbing in `client.go` lines 83–91 is already the right mechanism for tracking the AI bot user — no new code needed for multi-user filtering.

## Dependency changes

- Add: `modernc.org/sqlite` (pure Go SQLite).
- Remove (via `go mod tidy` after deletions): `github.com/docker/docker`, `k8s.io/client-go`, `k8s.io/apimachinery`, anything pulled only by docker/kube/log packages.

## Future work (out of scope for v1)

Captured here so it's not lost — revisit only after v1 is in daily use.

- **Tier 2 — Stream Claude responses inside lazydev**: a third pane that runs `claude -p` against the selected context, renders the streaming response, and captures the session to `~/.cache/lazydev/sessions/<ts>.md`. `R` on a Claude-authored MR runs a Claude review on its diff and offers to post the result as a comment.
- **Tier 3 — Full end-to-end automation**: `N` optionally spawns a Claude job in a configured working directory (detached or in a tmux pane); a watcher detects when Claude pushes a branch and auto-creates the MR; another watcher polls Claude MRs for new revisions and re-runs review. Off by default, gated behind `automation.spawn_claude: true` with a strict allow-list of working dirs.
- **Redaction**: regex pre-flight for tokens/JWTs/emails before clipboard or file write, with a status-bar diff (`redacted 3 secrets`). Defer until a leak or near-miss makes it concrete.
- **MCP server mode**: expose lazydev's cache as an MCP server so Claude Code can query issues/MRs directly via tool calls instead of having context pasted in.

## Verification

1. **Build clean**: `task check` (format + lint + build) passes on the branch.
2. **First-run cold start**: delete `~/.local/state/lazydev/cache.db`, run `./lazydev`. Issues sidebar must populate within ~1 s (full sync). Confirm DB created with expected tables (`sqlite3 cache.db .schema`).
3. **Warm start latency**: kill and re-launch with cache present. Sidebar must appear before any network call completes — verifiable by running with `GITLAB_TOKEN=invalid` after a successful sync: sidebar still renders from disk, sync status shows error.
4. **Prefetch sweep**: after first-run prefetch, query `sqlite3 cache.db 'SELECT COUNT(*), MIN(updated_at), MAX(updated_at) FROM issues'`. The min should be ~30 days ago, count should match GitLab's web filter `updated_after=<date>` for the project. Repeat for `mrs`.
5. **Offline mode**: disconnect network (`sudo ip link set <iface> down` or just `unset GITLAB_TOKEN`), launch lazydev. All Issues and MRs from the last 30 days are browseable. Detail panes render from cache. Status bar shows `offline · last sync <time>`. Reconnect → status flips to `idle` within one tick and any drift is pulled in.
6. **Janitor**: insert a row with `state='closed'` and `updated_at = now - 60d`, run the janitor manually (expose a debug `:gc` command or just trigger via test), verify the row is gone but open items of equal age are kept.
7. **Incremental sync**: in another terminal, edit an issue title in GitLab web UI. Within ~20s the lazydev sidebar updates without a manual refresh. Confirm only 1 list call was made (server-side log or pcap if available; or temporarily log `len(items)` in `ListIssuesUpdatedAfter`).
8. **Query DSL**: press `/`, type `assignee:@me label:bug state:open`. Sidebar filters to matching rows within ~30ms. Add a bare term (`refresh`) and verify FTS5 fuzzy match against body text. Press `Esc` — filter clears. Type `kind:mr assignee:@ai` and confirm MR results appear regardless of which tab is active.
9. **Saved views**: launch with empty `views.yaml`; verify defaults are written. `1` jumps to `mine`, `2` to `ai-queue`. `:save urgent label:urgent state:open` then `3` recalls it.
10. **Multi-select + copy-as-context**: `Space` on three issues → status bar shows `3 selected`. `y` → paste into a scratch buffer in another terminal and confirm valid XML with three `<issue>` blocks. `Y` → file exists at `/tmp/lazydev-ctx-*.md`. `Ctrl+Enter` → `claude -p` runs with the payload on stdin (replace `claude` with `cat > /tmp/test-stdin` in `llm_command` for the test).
11. **Deep pack**: select an MR-linked issue, `Shift+Y` → output includes the full MR body and diff stat fetched live from GitLab.
12. **AI handoff**:
    - `N` → create issue assigned to `ai_user` → switching to view `2` (`ai-queue`) shows it within one sync tick (or immediately via optimistic upsert).
    - `T` on it → reassigns to self → disappears from `ai-queue`, appears in `mine`.
13. **Write actions are still fast**: `s` to close an issue updates the sidebar within 200ms (optimistic cache upsert), no waiting for next sync tick.
14. **No regressions in write paths**: comment, approve, merge, assign-self still work — same code paths, only read paths changed.
