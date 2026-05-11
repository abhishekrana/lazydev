# lazydev rewrite — focus on Issues + MRs, SQLite cache, AI workflow

## Context

`lazydev` is currently a unified TUI for Docker / K8s / Logs / GitLab. The scope has drifted from how the tool is actually used: the daily-driver value is GitLab Issues + MRs, especially for triaging work between the human user and AI agents (Claude Code) acting as bot users. The Docker/K8s/Logs surfaces add ~1,700 LOC of code, dependencies (Docker SDK, client-go), and maintenance load with no current payoff.

Three problems to fix in this rewrite:

1. **Scope bloat** — delete everything except Issues + MRs (the GitLab client itself is mostly kept, minus pipelines).
2. **Latency on every interaction** — today every tab does a fresh API call on Init, on every 30s tick, and on detail open (3 calls per issue, 2 per MR). Nothing is cached. Sidebar takes 1–3 seconds to populate on every cold start and on every refresh tick.
3. **No offline mode** — pull the network and the app is useless. The user wants a local mirror of the last ~30 days of project activity so they can browse, search, and triage offline; when the network returns, the cache catches up automatically.

Goal: a focused, snappy, AI-workflow-aware TUI where the sidebar is instant from disk, the last 30 days of activity is browseable offline, background sync keeps things fresh, and the user can hand tickets/MRs between themselves and a Claude bot account fluently.

## Decisions (from clarifying questions)

- **Branch**: `rewrite-issues-mrs` off `main` (currently at `c0fc9a0`).
- **Cache**: SQLite via `modernc.org/sqlite` (pure Go, no CGO) at `~/.local/state/lazydev/cache.db`, with a thin in-memory map for the active view.
- **Sync model**: background sync every ~20s using `updated_after=MAX(updated_at)`; sidebar renders from SQLite on startup with zero network wait.
- **Prefetch window**: on first run (or whenever the cache is empty), backfill the last 30 days of _all_ issues and MRs in the configured project — not just items assigned to the tracked users. This gives a true offline mode and powers search across tickets the user didn't author. Window is configurable (`cache.prefetch_window_days`, default 30).
- **Retention**: closed/merged items older than the prefetch window are pruned by a background janitor. Open items are kept forever regardless of age — they're still actionable.
- **Layout**: keep separate Issues and MRs tabs.
- **Search**: SQLite FTS5 over title + description + notes; `/` opens a global cross-cache search overlay.
- **AI workflow**: all four features (quick-create, toggle assignee, AI Queue group, prompt-file export) — trim during implementation if any feel like overkill.
- **Scope cuts**: delete Pipelines, Docker, Kubernetes, Dashboard, Logs tabs and their backing packages.

## Step 1 — Cut scope

Create branch and delete in one commit:

- `git checkout -b rewrite-issues-mrs` (done).
- Delete tab files: `internal/ui/tabs/{pipelines,docker,kubernetes,dashboard,logs}.go` (~1,738 LOC).
- Delete packages: `internal/docker/`, `internal/kube/`, `internal/log/`, `internal/export/`, `internal/discovery/`.
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

## Step 4 — AI workflow features

All in Issues and (where it makes sense) MRs tabs:

- **Config**: add `ai_user` to GitLab config (single username, e.g. `claude-bot`). It must already be in `additional_users` to show in "My Issues".
- **`N` — Quick-create issue**: opens `$EDITOR` with template `# Title\n\n## Description\n`, parses on save, calls `gitlab.CreateIssue(title, body)` then `gitlab.AssignIssue(newIID, aiUserID)`. Reuses `InputModal`/editor plumbing already used by `c` (comment). After creation: upsert into cache, redraw sidebar.
- **`T` — Toggle assignee**: on the selected issue/MR, calls `AssignIssue` / equivalent on MR to flip assignee between current user and `ai_user`. Optimistic cache upsert.
- **`AI Queue` sidebar group**: in `groupIssues()` / `groupMRs()` (existing grouping logic in `tabs/issues.go` and `tabs/mergerequests.go`), add a top group `AI Queue` containing items where `assignee_username == cfg.GitLab.AIUser`, above the existing "Current Sprint" / "Backlog" groups. Pure local filter, no extra API cost.
- **`E` — Export selected to prompt file**: extend `Sidebar` with a multi-select mode (toggle via `v`, vim-style). `E` writes selected items to `~/.cache/lazydev/prompt.md` with a stable markdown layout (`## ISSUE-123: title`, then description, then notes). Path echoed in the notification bar so it can be pasted into Claude Code.

## Step 5 — Global search (`/`)

- `internal/ui/components/searchoverlay.go` — new component, modeled on `cmdpalette.go`. Bound to `/` globally in `internal/ui/root.go` (overrides today's sidebar-local `/` filter; keep the sidebar filter on `f` instead — already used for log filter which is being deleted, so it's free).
- Sends `SearchRequestMsg{Query: q}` to a small dispatcher in `root.go` which calls `state.Cache.Search(q, 50)` synchronously (SQLite FTS5 is sub-millisecond at this scale).
- Selecting a hit jumps to the right tab + selects the IID by sending the existing `TabActivatedMsg` plus a new `FocusItemMsg{Kind string, IID int}` that Issues/MRs tabs handle to set `selectedIID` and refresh the detail pane.

## Critical files to modify

- `cmd/lazydev/main.go` — open cache, start syncer, register fewer tabs.
- `internal/app/app.go` — slim `SharedState` to `{GitLabClient, Cache, Syncer, Config}`.
- `internal/config/config.go` — drop Docker/K8s/Logs, add `cache` + `ai_user`.
- `internal/ui/root.go` — register 2 tabs, bind `/` to global search overlay, route `CacheUpdatedMsg` and `FocusItemMsg`.
- `internal/ui/tabs/issues.go` — switch reads to cache, add `N` / `T` / `E` / AI Queue group, drop the 30s ticker.
- `internal/ui/tabs/mergerequests.go` — same.
- `internal/gitlab/issues.go` — add `ListIssuesUpdatedAfter`.
- `internal/gitlab/mergerequests.go` — add `ListMRsUpdatedAfter`.
- `internal/cache/{store.go,schema.go,sync.go}` — new.
- `internal/ui/components/searchoverlay.go` — new.
- `pkg/messages/messages.go` — add cache + search messages, remove pipeline/docker/k8s messages.

## Files / packages to delete

- `internal/ui/tabs/{pipelines,docker,kubernetes,dashboard,logs}.go`
- `internal/{docker,kube,log,export,discovery}/` (entire packages)
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

## Verification

1. **Build clean**: `task check` (format + lint + build) passes on the branch.
2. **First-run cold start**: delete `~/.local/state/lazydev/cache.db`, run `./lazydev`. Issues sidebar must populate within ~1 s (full sync). Confirm DB created with expected tables (`sqlite3 cache.db .schema`).
3. **Warm start latency**: kill and re-launch with cache present. Sidebar must appear before any network call completes — verifiable by running with `GITLAB_TOKEN=invalid` after a successful sync: sidebar still renders from disk, sync status shows error.
4. **Prefetch sweep**: after first-run prefetch, query `sqlite3 cache.db 'SELECT COUNT(*), MIN(updated_at), MAX(updated_at) FROM issues'`. The min should be ~30 days ago, count should match GitLab's web filter `updated_after=<date>` for the project. Repeat for `mrs`.
5. **Offline mode**: disconnect network (`sudo ip link set <iface> down` or just `unset GITLAB_TOKEN`), launch lazydev. All Issues and MRs from the last 30 days are browseable. Detail panes render from cache. Status bar shows `offline · last sync <time>`. Reconnect → status flips to `idle` within one tick and any drift is pulled in.
6. **Janitor**: insert a row with `state='closed'` and `updated_at = now - 60d`, run the janitor manually (expose a debug `:gc` command or just trigger via test), verify the row is gone but open items of equal age are kept.
7. **Incremental sync**: in another terminal, edit an issue title in GitLab web UI. Within ~20s the lazydev sidebar updates without a manual refresh. Confirm only 1 list call was made (server-side log or pcap if available; or temporarily log `len(items)` in `ListIssuesUpdatedAfter`).
8. **Search**: press `/`, type a substring from an issue body (not title). Hit appears, Enter focuses the right tab and selects the issue. Confirm the hit can be a closed issue from 3 weeks ago — i.e. search works across the full prefetch window, not just open items.
9. **AI workflow end-to-end**:
   - `N` → create issue assigned to `ai_user` → appears in `AI Queue` group within one sync tick.
   - `T` on it → reassigns to self → moves out of `AI Queue` into `Backlog`.
   - Select 3 issues with `v`, press `E` → `~/.cache/lazydev/prompt.md` contains all three formatted; notification bar shows the path.
10. **Write actions are still fast**: `s` to close an issue updates the sidebar within 200ms (optimistic cache upsert), no waiting for next sync tick.
11. **No regressions in write paths**: comment, approve, merge, assign-self still work — same code paths, only read paths changed.
