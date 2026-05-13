# CLAUDE.md

## Project Overview

**lazydev** is a terminal cockpit for working GitLab issues and MRs with Claude Code as a pair programmer. Three tabs — Issues, MRs, Claude (sessions). All reads come from a local SQLite mirror; a background Syncer keeps the cache fresh. The query DSL on `/`, multi-select export, saved views, and `C` / `P` Claude dispatch are the differentiating features. Built with Go and Bubble Tea v2; Solarized Light by default.

## Setup & Build

```bash
./bootstrap.sh   # install Taskfile runner
task init        # install dev tools (goimports, golangci-lint) and tidy
task build       # build binary (./lazydev)
task install     # install to ~/.local/bin
task run         # build + run
task tidy        # go mod tidy
task format      # format Go, Markdown, YAML
task lint        # golangci-lint
task check       # format + lint + build (run before committing)
go build ./...   # compile-check all packages
```

## Project Structure

- `cmd/lazydev/main.go` — entry: loads config, builds `SharedState`, wires `Syncer.Events()` into the Bubble Tea program before `StartSync`, constructs `Issues`/`MRs`/`Claude` tabs.
- `internal/app/app.go` — `SharedState`: `GitLabClient`, `Cache` (SQLite store), `Syncer`, `Views`, `Config`, `ClaudeEnv`, `ClaudeStore`. Fails closed if GitLab isn't configured; warns (does not fail) when `claude` isn't on PATH.
- `internal/cache/` — SQLite mirror (`modernc.org/sqlite`, pure Go, no CGo). `store.go` opens/migrates the DB; `schema.go` is the migrations; `search.go` wraps FTS5; `filter.go` is the `Filter` struct used by `ListIssues`/`ListMRs`; `sync.go` is the `Syncer` (prefetch window + periodic `updated_after` polling, publishes `SyncEvent` over a channel).
- `internal/claude/` — Claude Code integration. `discovery.go` resolves `claude`/`tmux` binaries and the repo root; `prompt.go` composes the structured prompt from `ExportItem`s; `dispatch.go` runs one-shot (`claude -p`, output to `.lazydev/claude-runs/<id>.log`) or interactive (tmux new-window/new-session); `session.go` persists dispatched sessions to `.lazydev/sessions.json`.
- `internal/query/parser.go` — DSL parser. `assignee:@me label:bug state:open kind:mr updated:>7d` plus bare fuzzy terms. `@me` → authenticated user, `@ai` → `cfg.GitLab.AIUser`, `@none` → unassigned sentinel, `@any` → no filter. Tokens are AND'd; quoted strings preserved by `tokenize`.
- `internal/views/views.go` — saved-view YAML store at `~/.config/lazydev/views.yaml`. Number keys `1`–`9` recall by index. Defaults seeded on first run: `mine`, `ai-queue`, `review`, `recent`. Atomic write on every Save/Delete.
- `internal/gitlab/` — GitLab API: `client.go` (auth discovery, project resolution from git remote, user ID resolution), `issues.go`, `mergerequests.go`, `sync.go` (`ListIssuesUpdatedAfter`, `ListMRsUpdatedAfter` for the cache).
- `internal/export/` — `context.go` builds Claude-XML or markdown bundles from `ExportItem`s; `export.go` writes `/tmp/lazydev-*` files and emits OSC52 clipboard escapes; `tty.go` / `tty_windows.go` handle writing OSC52 to the controlling terminal.
- `internal/config/` — YAML config + defaults. XDG-compliant paths: config at `~/.config/lazydev/config.yaml`, cache at `~/.local/state/lazydev/cache.db`.
- `internal/ui/root.go` — `RootModel`, tab dispatch, help overlay, command palette (`:save`, `:view`, `:del`, `:tab`, `:help`, `:q`), view-recall on `1`–`9`. Broadcasts the data messages listed below to all tabs.
- `internal/ui/components/` — `Sidebar` (grouped, multi-select with `Space`/`v`/`Esc`, `/` search), `DetailPane` (Ctrl+click URLs), `QueryLine`, `Modal`, `InputModal`, `HelpOverlay`, `CmdPalette`, `TabBar`, `StatusBar`.
- `internal/ui/tabs/` — `IssuesTab`, `MRsTab`, `ClaudeTab`, plus `options.go` (shared `Options` bundle) and `claude_dispatch.go` (`dispatchClaude` helper called from `C` / `P` in both Issues and MRs).
- `pkg/messages/messages.go` — all cross-package `tea.Msg` types (broadcast set listed below).

## Key Architecture Decisions

- **Bubble Tea v2 API**: `Init()` has no args, `View()` returns `tea.View` (not `string`), use `tea.KeyPressMsg` (not `tea.KeyMsg`). `AltScreen` and `MouseMode` are set on the `tea.View` struct in `RootModel.View()`.
- **Cache-first reads**: tabs render from `cache.Store` on `Init()` with no network wait. The Syncer (started by `main.go` after the event-forwarding goroutine is wired up) does an initial prefetch over `cfg.Cache.PrefetchWindowDays`, then periodic `updated_after` polls. It publishes `SyncEvent` over a channel; `main.go`'s `forwardSyncEvents` converts each to `SyncStatusMsg` (status) and `CacheUpdatedMsg` (data) and `p.Send`s them into the program.
- **Detail-pane fetch pattern**: `selectIssue` / `selectMR` returns `tea.Batch(cacheCmd, apiCmd)`. The cache result paints immediately; the API result (1–2s later) overwrites it and upserts back to the cache. Both share an incrementing `fetchSeq` so stale responses are discarded.
- **Sidebar debounce**: cursor movement schedules a `tea.Tick(150ms)` carrying the new item ID; the tick only triggers a detail fetch if `pendingFetch` still equals that ID — keeps rapid `j`/`k` from spamming GitLab.
- **Multi-select**: `components.Sidebar` owns `marked map[string]bool` and `visualStart`. Tabs read it via `Marked()` when building export bundles, falling back to the cursor item if nothing is marked (`buildExportItems` in each tab).
- **Tab activation**: root sends `messages.TabActivatedMsg` when switching. Tabs that need deferred work after a list arrives (e.g. auto-selecting the first item) set a `needsAutoSelect` flag in the list handler and act on it in `TabActivatedMsg` — never return commands producing local message types from broadcast handlers, since the result is dropped if the tab isn't active.
- **Broadcast set**: `RootModel.Update` broadcasts these messages to every tab so background tabs stay current: `ExecFinishedMsg`, `IssueListMsg`, `IssueDetailMsg`, `IssueActionMsg`, `MRListMsg`, `MRDetailMsg`, `MRActionMsg`, `CacheUpdatedMsg`, `SyncStatusMsg`, `ApplyViewMsg`, `ClaudeDispatchMsg`, `ClaudeSessionsReloadMsg`. All other messages route only to the active tab.
- **TabModel interface** (`internal/ui/root.go`): `Init()`, `Update() (TabModel, tea.Cmd)`, `View() string`, `Title()`, `SetSize()`. Optional `Notifier` interface lets tabs surface status-bar messages.
- **GitLab auth discovery**: config → `GITLAB_TOKEN` env → `~/.config/glab-cli/config.yml` (handles `!!null` YAML tag). Project auto-detected from `git remote get-url origin`. App refuses to start if no GitLab client is built.
- **Multi-user tracking**: queries fan out across `cfg.GitLab.AdditionalUsers` plus the authenticated user; sidebar grouping uses the union to decide "Assigned to me/bot" vs "Other".
- **Claude dispatch (`C` / `P`)**: shared `dispatchClaude` in `tabs/claude_dispatch.go` builds a structured prompt via `claude.Compose`, then calls `claude.DispatchInteractive` (tmux new-window inside tmux, new-session outside) or `claude.DispatchOneShot` (`claude -p`, output to `.lazydev/claude-runs/<id>.log`). Both persist a `Session` record to `.lazydev/sessions.json` (`claude.Store`). The `ClaudeTab` lists those records and re-attaches via `tmux attach`.
- **Query DSL → cache**: the queryline emits an `Expression`; `Filter` goes straight to `cache.ListIssues`/`ListMRs`; `UpdatedAfter`/`UpdatedBefore` from `updated:` tokens narrow the result set; `Kind` ("issue" / "mr") short-circuits the wrong tab to an empty list so a single expression can target either tab. Unknown keys fall through as fuzzy text rather than errors.
- **Saved views (1–9)**: `RootModel` checks `views.ByIndex(idx)` first; if present it sends `ApplyViewMsg` to the active tab (which sets `queryExpr` and refetches). If no view exists at that index AND the index maps to a real tab, it falls back to tab switching.
- **Two-key sequences**: `gg` (top) uses `pendingG`; `Ctrl+W w` (pane toggle) uses `pendingCtrlW`. Both reset on any unrelated keypress.

## Rules

- **Never commit personal info**: no names, emails, IP addresses, tokens, or company references.
- **Solarized Light**: test that text is readable on a light background.
- **Keep scope tight**: this repo deliberately dropped Docker/K8s/Logs/Dashboard/Pipelines (commit `e08cd1f`). Don't reintroduce them. Three tabs only.
- **Keep code consistent across the GitLab tabs** (Issues, MRs):
  - Struct fields: `client`, `store`, `syncer`, `opts`, `sidebar`, `detailPane`, `queryline`, `modal` (+ `inputModal` for Issues), `focusSidebar`, `width`, `height`, `selectedIID`, item slice, `queryExpr`, `notification`, `pendingCtrlW`, `fetchSeq`, `pendingFetch`, `needsAutoSelect`.
  - `Init()` returns `fetchIssues()` / `fetchMRs()` (cache read, no network).
  - `Update()` handles: list msg → populate sidebar + flag `needsAutoSelect`; detail-fetch tick → kick off `selectX`; detail result → set detail pane (discard stale `seq`); action msg → notification + `syncer.SyncNow()` + refetch; `CacheUpdatedMsg{Kind:…}` → refetch; `ApplyViewMsg` → set `queryExpr` + refetch; `ClaudeDispatchMsg` → notification; `ExportDoneMsg` → notification.
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
- Config path: `~/.config/lazydev/config.yaml`. Views: `~/.config/lazydev/views.yaml`. Cache: `~/.local/state/lazydev/cache.db`. Claude dispatch artifacts: `.lazydev/sessions.json`, `.lazydev/claude-runs/`, `.lazydev/claude-prompts/` (repo-relative).
- Markdown in detail panes is rendered via glamour with `WithWordWrap(paneWidth)`.
- Relative GitLab URLs are resolved to absolute; `/uploads/` paths use `/-/project/{id}/uploads/` format.
- Export bundles default to `claude-xml` (Anthropic's recommended multi-document framing); switch to `markdown` via `export.format` when piping to non-Claude tools.

## Current Status

Issues/MRs focus + SQLite cache + Claude Code handoff is the v2 product. Recent additions:

- **Claude Code integration** (`60db54e`) — `C` interactive, `P` one-shot, sessions tab, `.lazydev/sessions.json` persistence.
- **Quick-create-for-AI and toggle assignee** (`4471d22`) — `N` and `T` keybindings.
- **Multi-select + Claude context export** (`ea44fba`) — `Space`/`v`/`Esc` marking, `y`/`Y`/`X` export.
- **Saved views with number-key recall** (`4cfa24a`).
- **Query DSL + queryline** (`75ab938`) — `/` opens the DSL prompt; tabs render from filtered cache.
- **SQLite cache + Syncer** (`14dc32a`, `2fc7761`) — replaces in-memory state and the 30 s tick; FTS5 fuzzy search.
- **Scope cut** (`e08cd1f`) — Docker/K8s/Logs/Dashboard/Pipelines removed.
