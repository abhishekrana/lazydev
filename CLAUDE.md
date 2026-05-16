# CLAUDE.md

## Project Overview

**lazydev** is a terminal cockpit for working GitLab issues and MRs with Claude Code as a pair programmer. Three tabs — Issues, MRs, Claude (sessions). All reads come from a local SQLite mirror; a background Syncer keeps the cache fresh. The query DSL on `/`, multi-select export, and `C` / `P` Claude dispatch are the differentiating features. Built with Go and Bubble Tea v2; Solarized Light by default.

## Setup & Build

```bash
./bootstrap.sh    # install Taskfile runner
task init         # install dev tools (goimports, golangci-lint) and tidy
task build        # build binary (./lazydev)
task install      # install to ~/.local/bin
task run          # build + run
task tidy         # go mod tidy
task format       # format Go, Markdown, YAML
task lint         # golangci-lint
task check        # format + lint + build (run before committing)
task wipe-cache   # rm ~/.local/state/lazydev/cache.db* (forces a fresh prefetch on next run)
go build ./...    # compile-check all packages
```

## Project Structure

- `cmd/lazydev/main.go` — entry: dispatches subcommands (`search`, `issue`, `mr`, `install-skill`) before falling through to `runTUI`. TUI path loads config, builds `SharedState`, wires `Syncer.Events()` into the Bubble Tea program before `StartSync`, constructs `Issues`/`MRs`/`Claude` tabs.
- `cmd/lazydev/cli_*.go` — read-only CLI surface over the cache. `cli_output.go` (helpers: `openCache`, `writeJSON`/`writeList`, `usage`/`writeLines`, `reorderFlags` so positionals can appear before or after flags); `cli_query.go` (`parseQuery` resolves `@me` from the `gitlab_username` row in `meta`, so the CLI never hits GitLab); `cli_search.go`, `cli_issues.go`, `cli_mrs.go` (per-DTO `*JSON` structs with `snake_case` tags — JSON contract lives in the CLI layer, `pkg/messages` stays untouched); `cli_skill.go` + `//go:embed skill.md` for `install-skill`. Subcommand list output is NDJSON by default; `--pretty` flips to indented JSON arrays.
- `cmd/lazydev/skill.md` — Claude Code skill body, compiled into the binary via `//go:embed`. Not delivered as a repo file to end users — `lazydev install-skill` writes it to `~/.claude/skills/lazydev/SKILL.md`.
- `internal/app/app.go` — `SharedState`: `GitLabClient`, `Cache` (SQLite store), `Syncer`, `Config`, `ClaudeEnv`, `ClaudeStore`. Fails closed if GitLab isn't configured; warns (does not fail) when `claude` isn't on PATH. Persists the authenticated GitLab username into the cache's `meta` table (`gitlab_username`) on startup so the read-only CLI subcommands can resolve `@me` in the query DSL without their own credential.
- `internal/cache/` — SQLite mirror (`modernc.org/sqlite`, pure Go, no CGo).
  - `schema.go` — `schemaSQL` defines `issues` / `mrs` / `notes` / `related_mrs` / `linked_items` / `child_items` / `meta` / `search_fts` (FTS5). `issues` carries `status` / `parent_iid` / `parent_title` / `assignees` (JSON) for the work-item widgets.
  - `store.go` — open + scan + upsert. `currentSchemaVersion` and `migrate()` drop the data tables on a version mismatch; on next launch the Syncer repopulates from GitLab. New helpers: `UpsertLinkedItems` / `ListLinkedItems` / `UpsertChildItems` / `ListChildItems`.
  - `sync.go` — `Source` interface (`ListMRsUpdatedAfter`, `StreamWorkItemsUpdatedAfter`) and `Syncer` (initial prefetch + 60 s ticks; publishes `SyncEvent` over a channel). Prefetch decision: empty cache ⇒ prefetch; otherwise incremental from `MaxIssueUpdatedAt` / `MaxMRUpdatedAt`.
  - `filter.go` — `Filter` consumed by `ListIssues` / `ListMRs`; assignee match is JSON-LIKE against the `assignees` array.
  - `search.go` — FTS5 helpers.
- `internal/gitlab/` — GitLab API.
  - `client.go` — auth discovery, project resolution from git remote, user ID resolution, **numeric `ProjectNumericID`** resolved at init for /uploads/ rewrites.
  - `issues.go` — `GetIssue` returns `(issue, notes, relatedMRs, linkedItems, childItems, error)`; uses `GetWorkItemBundle` (GraphQL) for the issue body + widgets and `c.Raw.Notes.ListIssueNotes` + `ListMergeRequestsRelatedToIssue` (REST) for the rest. `FormatIssueDetail` renders the header strip + body + footer sections.
  - `mergerequests.go` — REST MR sync + detail formatting.
  - `workitems_graphql.go` — bulk + single-item GraphQL via `c.Raw.GraphQL.Do`. `StreamWorkItemsUpdatedAfter(t, onPage)` paginates `namespace.workItems` 50 at a time with every widget (`Description`, `Assignees`, `Labels`, `Milestone`, `Iteration`, `Status`, `Hierarchy`, `LinkedItems`) and yields a `messages.WorkItemPage` per page. `GetWorkItemBundle(iid)` is the per-detail variant.
  - `sync.go` — `ListMRsUpdatedAfter` (REST). Issue sync is GraphQL now.
  - `format.go` — `linkify` (OSC 8 + SGR underline + SolBlue), `formatChildItems`, `formatLinkedItems`, `formatParent` (derives parent URL via IID substitution), `formatDateWithAge`, header-strip helpers.
- `internal/claude/` — Claude Code integration. `discovery.go` resolves `claude`/`tmux` binaries and the repo root; `prompt.go` composes the structured prompt from `ExportItem`s; `dispatch.go` runs one-shot (`claude -p`, spawned in the background, output to `.lazydev/claude-runs/<id>.log` mode `0600`; a goroutine waits on exit and flips the session record) or interactive (writes a `/bin/sh` launcher script then asks tmux to `new-window`/`new-session sh <path>` — keeps dispatch working under fish/nushell); `session.go` persists dispatched sessions to `.lazydev/sessions.json`. `finishSession` helper unifies the done/failed status update on both Start-failed and Wait-completed paths.
- `internal/query/parser.go` — DSL parser. `assignee:@me label:bug state:open kind:mr updated:>7d` plus bare fuzzy terms. `@me` → authenticated user, `@ai` → `cfg.GitLab.AIUser`, `@none` → unassigned sentinel, `@any` → no filter. Tokens are AND'd; quoted strings preserved by `tokenize`.
- `internal/export/` — `context.go` builds Claude-XML or markdown bundles from `ExportItem`s; `export.go` writes `/tmp/lazydev-*` files and emits OSC52 clipboard escapes; `tty.go` / `tty_windows.go` handle writing OSC52 to the controlling terminal.
- `internal/config/` — YAML config + defaults. XDG-compliant paths: config at `~/.config/lazydev/config.yaml`, cache at `~/.local/state/lazydev/cache.db`. Default `cache.sync_interval_s = 60`.
- `internal/ui/root.go` — `RootModel`, tab dispatch, help overlay, command palette (`:tab`, `:help`, `:q`), tab switching on `1`–`9`. Tracks the latest `SyncStatusMsg` and renders it as the right-side status-bar indicator (`prefetching N…` / `synced 5s ago` / `offline: <err>`). Broadcasts the data messages listed below to all tabs.
- `internal/ui/components/` — `Sidebar` (grouped, multi-select with `Space`/`v`/`Esc`, `/` search), `DetailPane` (bold flush-left title + spacer row + scrollable content; Ctrl+click resolves an OSC 8 or plain URL on the clicked row with ±1 row tolerance), `QueryLine` (Esc clears, Enter commits while keeping the filter applied), `Modal`, `InputModal`, `HelpOverlay`, `CmdPalette`, `TabBar`, `StatusBar` (key hints + sync indicator).
- `internal/ui/tabs/` — `IssuesTab`, `MRsTab`, `ClaudeTab`, plus `options.go` (shared `Options` bundle + `containsString` helper for multi-assignee comparisons) and `claude_dispatch.go` (`dispatchClaude` helper called from `C` / `P` in both Issues and MRs).
- `pkg/messages/messages.go` — all cross-package `tea.Msg` types (broadcast set listed below). `GitLabIssue` carries `Status` / `ParentIID` / `ParentTitle` / `Assignees []string`; new types `GitLabLinkedItem`, `GitLabChildItem`, `WorkItemPage`.

## Key Architecture Decisions

- **Bubble Tea v2 API**: `Init()` has no args, `View()` returns `tea.View` (not `string`), use `tea.KeyPressMsg` (not `tea.KeyMsg`). `AltScreen` and `MouseMode` are set on the `tea.View` struct in `RootModel.View()`.
- **Cache-first reads**: tabs render from `cache.Store` on `Init()` with no network wait. The Syncer (started by `main.go` after the event-forwarding goroutine is wired up) does an initial prefetch over `cfg.Cache.PrefetchWindowDays`, then periodic `updated_after` polls. It publishes `SyncEvent` over a channel; `main.go`'s `forwardSyncEvents` converts each to `SyncStatusMsg` (status) and `CacheUpdatedMsg` (data) and `p.Send`s them into the program.
- **Empty-cache prefetch trigger**: the Syncer decides "should I prefetch on startup?" by checking `MaxIssueUpdatedAt.IsZero() && MaxMRUpdatedAt.IsZero()`. No separate `last_full_sync` watermark — derived from the data itself, so a schema-drop never leaves the syncer out of sync with the (now-empty) tables. Force a fresh prefetch via `task wipe-cache`.
- **Schema versioning**: `cache.currentSchemaVersion` (string) stored in `meta`. `migrate()` drops data tables on mismatch and lets `schemaSQL` recreate them; the Syncer repopulates from GitLab next start. Acceptable because the cache is a mirror, not the source of truth.
- **GraphQL bulk fetch for issues**: one paginated `namespace.workItems(types: [ISSUE], first: 50, after: …, updatedAfter: …)` query returns title/state + every widget (Description, Assignees, Labels, Milestone, Iteration, Status, Hierarchy{parent,children}, LinkedItems). Replaces ~3N REST calls with ~N/50 GraphQL calls. Same query, single-item variant (`iids: [...]`), is reused for the per-detail freshness path. MR sync is still REST.
- **Detail-pane fetch pattern**: `selectIssue` returns `tea.Batch(cacheCmd, apiCmd)`. Cache paints immediately (issue row + notes + related MRs + linked items + child items); API result (1–2 s later) overwrites and upserts back to the cache. Both share an incrementing `fetchSeq` so stale responses are discarded.
- **Detail pane chrome + click**: title row is bold, flush-left (no horizontal padding); blank spacer row below it. Click handler translates `mouse.Y - yOffset - 2` to a content row, then runs `urlOnLine` (OSC 8 regex first, then plain http(s)); falls back to ±1 row to tolerate alignment drift between rendered visual rows and `\n`-split lines.
- **OSC 8 hyperlinks**: every reference (`#NNN` / `!NNN`) and the URL row are wrapped via `gitlab.linkify(text, url)` — OSC 8 open + SGR underline + SolBlue + text + SGR reset + OSC 8 close. The terminal renders them underlined and clickable; lazydev's `Ctrl+click` extracts the URL from the OSC 8 escape on the clicked row.
- **Status-bar sync indicator**: `RootModel.sync` stores the latest `SyncStatusMsg`; `formatSyncIndicator` renders `starting…` (pre-event) / `prefetching N…` / `syncing…` / `synced 5s ago` (green) / `offline: <err>` (red) on the right of the status bar.
- **Sidebar debounce**: cursor movement schedules a `tea.Tick(150ms)` carrying the new item ID; the tick only triggers a detail fetch if `pendingFetch` still equals that ID — keeps rapid `j`/`k` from spamming GitLab.
- **Multi-select**: `components.Sidebar` owns `marked map[string]bool` and `visualStart`. Tabs read it via `Marked()` when building export bundles, falling back to the cursor item if nothing is marked (`buildExportItems` in each tab).
- **Multi-assignee**: `GitLabIssue.Assignees []string` (was scalar). Cache stores as JSON; `Filter.Assignee` selects via `LIKE '%"<user>"%'`. Tab AI-toggle uses `containsString(issue.Assignees, opts.AIUser)`. Export bundles join with `, `.
- **Tab activation**: root sends `messages.TabActivatedMsg` when switching. Tabs that need deferred work after a list arrives (e.g. auto-selecting the first item) set a `needsAutoSelect` flag in the list handler and act on it in `TabActivatedMsg` — never return commands producing local message types from broadcast handlers, since the result is dropped if the tab isn't active.
- **Broadcast set**: `RootModel.Update` broadcasts these messages to every tab so background tabs stay current: `ExecFinishedMsg`, `IssueListMsg`, `IssueDetailMsg`, `IssueActionMsg`, `MRListMsg`, `MRDetailMsg`, `MRActionMsg`, `CacheUpdatedMsg`, `SyncStatusMsg`, `ClaudeDispatchMsg`, `ClaudeSessionsReloadMsg`. All other messages route only to the active tab.
- **TabModel interface** (`internal/ui/root.go`): `Init()`, `Update() (TabModel, tea.Cmd)`, `View() string`, `Title()`, `SetSize()`. Optional `Notifier` interface lets tabs surface status-bar messages.
- **GitLab auth discovery**: config → `GITLAB_TOKEN` env → `~/.config/glab-cli/config.yml` (handles `!!null` YAML tag). Project auto-detected from `git remote get-url origin`. `Client.ProjectNumericID` resolved at init via `Projects.GetProject` for /uploads/ rewrites. App refuses to start if no GitLab client is built.
- **Multi-user tracking**: queries fan out across `cfg.GitLab.AdditionalUsers` plus the authenticated user; sidebar grouping uses the union to decide "Assigned to me/bot" vs "Other".
- **Claude dispatch (`C` / `P`)**: shared `dispatchClaude` in `tabs/claude_dispatch.go` builds a structured prompt via `claude.Compose`, then calls `claude.DispatchInteractive` or `claude.DispatchOneShot`. Both return immediately and persist a `Session` record (status `running`) to `.lazydev/sessions.json` (`claude.Store`). `DispatchInteractive` writes a `/bin/sh` launcher script to `.lazydev/claude-prompts/<id>.sh` and asks tmux to run `sh <path>` via `new-window` (inside tmux) or `new-session` (outside) — going through `/bin/sh` rather than the user's `$SHELL` keeps POSIX `'\''` quoting working under fish/nushell. `DispatchOneShot` spawns `claude -p` in the background; a goroutine waits on exit and calls `finishSession` to flip status to `done`/`failed`. The `ClaudeTab` lists those records and re-attaches via `tmux attach`.
- **Query DSL → cache**: the queryline emits an `Expression`; `Filter` goes straight to `cache.ListIssues`/`ListMRs`; `UpdatedAfter`/`UpdatedBefore` from `updated:` tokens narrow the result set; `Kind` ("issue" / "mr") short-circuits the wrong tab to an empty list so a single expression can target either tab. Unknown keys fall through as fuzzy text rather than errors. Enter on the queryline hides the line but keeps `queryExpr` applied; Esc clears.
- **Two-key sequences**: `gg` (top) uses `pendingG`; `Ctrl+W w` (pane toggle) uses `pendingCtrlW`. Both reset on any unrelated keypress.

## Rules

- **Never commit personal info**: no names, emails, IP addresses, tokens, or company references.
- **Solarized Light**: test that text is readable on a light background.
- **Keep scope tight**: this repo deliberately dropped Docker/K8s/Logs/Dashboard/Pipelines (commit `e08cd1f`). Don't reintroduce them. Three tabs only.
- **Keep code consistent across the GitLab tabs** (Issues, MRs):
  - Struct fields: `client`, `store`, `syncer`, `opts`, `sidebar`, `detailPane`, `queryline`, `modal` (+ `inputModal` for Issues), `focusSidebar`, `width`, `height`, `selectedIID`, item slice, `queryExpr`, `notification`, `pendingCtrlW`, `fetchSeq`, `pendingFetch`, `needsAutoSelect`.
  - `Init()` returns `fetchIssues()` / `fetchMRs()` (cache read, no network).
  - `Update()` handles: list msg → populate sidebar + flag `needsAutoSelect`; detail-fetch tick → kick off `selectX`; detail result → set detail pane (discard stale `seq`); action msg → notification + `syncer.SyncNow()` + refetch; `CacheUpdatedMsg{Kind:…}` → refetch; `ClaudeDispatchMsg` → notification; `ExportDoneMsg` → notification.
  - Sidebar keys: `Enter` open detail, `o` open browser, `s` close/reopen toggle, `c` comment via `$EDITOR`. Plus `a` (Issues only), `T` toggle AI assignee (both), `N` quick-create-for-AI (Issues only), `R`/`m`/`A` (MRs only).
  - Global keys per tab: `/` query line, `r` refresh (nudges syncer), `y`/`Y`/`X` export, `C`/`P` Claude dispatch, `Ctrl+W W` / `Alt+W` pane focus toggle.
  - Detail fetch debounce: 150 ms tick, `pendingFetch` + `fetchSeq` for stale-response rejection.
  - Destructive actions use the confirmation modal (`modal.Show` with callback).
  - When adding a feature to one tab, check if it applies to the other and add it there too.

## Conventions

- **Do not add Co-Authored-By lines to commit messages.**
- **Plans before implementation**: write the plan to `docs/` on the feature branch and commit before starting work (see auto-memory `Plan docs workflow`).
- Keybindings accept both vim (`hjkl`) and arrow keys via `key.NewBinding` with multiple keys.
- Sidebar width is 25 % of terminal width (minimum 30 cols).
- Config path: `~/.config/lazydev/config.yaml`. Cache: `~/.local/state/lazydev/cache.db`. Claude dispatch artifacts (repo-relative): `.lazydev/sessions.json` (ledger), `.lazydev/claude-runs/<id>.log` (one-shot output, mode `0600`), `.lazydev/claude-prompts/<id>.md` (composed prompt) and `<id>.sh` (launcher script).
- Markdown in detail panes is rendered via glamour with `WithWordWrap(paneWidth)`.
- Relative GitLab URLs are resolved to absolute; `/uploads/` paths use `/-/project/{id}/uploads/` format.
- Export bundles default to `claude-xml` (Anthropic's recommended multi-document framing); switch to `markdown` via `export.format` when piping to non-Claude tools.

## Current Status

Issues/MRs focus + SQLite cache + Claude Code handoff is the v2 product. Recent changes (newest first):

- **Cache CLI + embedded Claude skill** (`f581429`) — five read-only subcommands (`lazydev search`, `lazydev issue list|show`, `lazydev mr list|show`) emit JSON / NDJSON over the same `cache.Store` + query DSL. The TUI now stashes `gitlab_username` in `meta` on startup so the CLI can resolve `@me` without a GitLab credential. `lazydev install-skill` writes a `//go:embed`'d `SKILL.md` to `~/.claude/skills/lazydev/`, keeping single-binary distribution (no repo files required on the end user's machine). Designed for cross-repo Claude Code sessions to pull issue/MR context from the same local cache the TUI is keeping fresh.
- **Cut saved views** (`be28ecf`) — dropped `internal/views/` package, `ApplyViewMsg`, palette commands `:save`/`:view`/`:del`, number-key view-recall. Query DSL on `/` covers the same use case; number keys `1`–`9` now just switch tabs.
- **Claude dispatch hardening** — one-shot runs in background (`09efe9a`), interactive uses `/bin/sh` launcher script for fish/nushell portability (`1d3da6a`), log file mode `0600` (`abcbc57`), unified `finishSession` for done/failed (`76ab912`).
- **Syncer cleanup** — dropped dead `stopped` atomic (`1dfe5bd`); documented the partial-prefetch self-heal (`2479075`).
- **OSC 8 + per-row Ctrl+click** (`ee15110`, `4afeab0`, `7bc79f9`, `0134e37`) — every `#NNN` / `!NNN` reference and URL is a clickable, underlined-blue hyperlink; click handler tolerates ±1 row drift.
- **GraphQL bulk fetch + work-item widgets** (`b5b2788`, `d6c8076`, `18366eb`, `bdc28b8`) — schema v3 (status, parent, linked_items, child_items); `namespace.workItems` with widgets replaces REST issue sync.
- **Multi-assignee** (`626629d`) — `GitLabIssue.Assignees []string`, JSON column in cache, propagated through UI/export/AI-toggle.
- **Detail-pane header strip** (`03ade6f`, `dad4b5c`, `63cbb1d`, `65d15d0`, `95210c6`) — gh-style header rows with `(Xd ago)` dates; flush-left bold title; blank spacer; `—` placeholders for empty fields.
- **Status-bar sync indicator** (`c5668c2`) — `starting…` / `prefetching N…` / `synced 5s ago` / `offline: <err>`.
- **QueryLine commit-on-Enter** (`14de496`) — Enter hides the line while keeping the filter applied; Esc still clears.
- **Sync simplification** (`06e1961`, `9c24142`) — dropped `last_full_sync`; empty-cache check drives prefetch; added `task wipe-cache`; default sync 60 s configurable via `cache.sync_interval_s`.
- **Claude Code integration** (`60db54e`) — `C` interactive, `P` one-shot, sessions tab.
- **SQLite cache + Syncer + Query DSL** — the v2 backbone.
- **Scope cut** (`e08cd1f`) — Docker/K8s/Logs/Dashboard/Pipelines removed.
