# CLAUDE.md

## Project Overview

**lazydev** is a unified TUI for Docker, Kubernetes, and GitLab — view logs, monitor status, manage resources, track issues/MRs/pipelines in one terminal tool. Built with Go and Bubble Tea v2. Designed for LLM-assisted debugging workflows.

## Setup & Build

```bash
./bootstrap.sh  # Install Taskfile runner
task init        # Install dev tools (goimports, golangci-lint) and tidy deps
task build       # Build binary (output: ./lazydev)
task install     # Install binary to ~/.local/bin
task run         # Build and run
task clean       # Remove binary
task tidy        # go mod tidy
task format      # Format Go, Markdown, YAML files
task lint        # Run golangci-lint
task check       # Format + lint + build (run before committing)
go build ./...   # Build all packages (check compilation)
```

## Project Structure

- `cmd/lazydev/main.go` — Entry point, creates SharedState and root model
- `internal/app/app.go` — SharedState: holds Docker/K8s clients, StreamManager, config
- `internal/ui/root.go` — Root Bubble Tea model, handles tab switching, message broadcasting to all tabs
- `internal/ui/theme/styles.go` — Solarized Light palette, all Lip Gloss styles
- `internal/ui/theme/keys.go` — All keybindings (vim + arrow keys) to avoid import cycles
- `internal/ui/components/` — Reusable widgets: TabBar, Sidebar, LogView, StatusBar, Modal, InputModal, DetailPane, Table, HelpOverlay, CmdPalette
- `internal/ui/tabs/` — Tab models: DockerTab, KubeTab, LogsTab, DashboardTab, IssuesTab, MRsTab, PipelinesTab
- `internal/gitlab/` — GitLab API wrapper: client (auth discovery from config/env/glab CLI), issues, merge requests, pipelines
- `internal/ui/layout/` — Split pane layout helpers
- `internal/docker/` — Docker SDK wrapper: client, container list/logs/inspect/stats, actions, compose grouping
- `internal/kube/` — Kubernetes client-go wrapper: pods, deployments, services, events, describe, scale
- `internal/log/` — Log subsystem: StreamManager (goroutine lifecycle), RingBuffer, filter, highlight, Docker header stripping, ANSI stripping
- `internal/export/` — Log export: LinesToText, LinesToJSON, ToFile (/tmp), ToClipboardOSC52
- `internal/config/` — YAML config struct and defaults
- `internal/discovery/` — Auto-detect Docker daemon and kubeconfig at startup
- `pkg/messages/` — All shared `tea.Msg` types used across packages

## Key Architecture Decisions

- **Bubble Tea v2 API**: `Init()` has no args, `View()` returns `tea.View` (not string), use `tea.KeyPressMsg` (not `tea.KeyMsg`), `AltScreen` is set on `tea.View` struct
- **Import cycle prevention**: Styles and keys live in `internal/ui/theme/` — components import `theme`, not `ui`
- **Log streaming**: Goroutines per source → fan-in channel → batched delivery (50ms/100 lines) → `LogBatchMsg` to Bubble Tea. Ring buffer bounds memory at 10k lines per source. Docker multiplexed stream headers and ANSI escape codes are stripped at the stream level.
- **Message broadcasting**: Root model broadcasts data messages (LogBatchMsg, ContainerListMsg, ResourceStatsMsg, etc.) to ALL tabs, not just the active tab — ensures background tabs stay current.
- **TabModel interface**: Defined in `internal/ui/root.go` — `Init()`, `Update()`, `View()`, `Title()`, `SetSize()`. Each tab returns `(ui.TabModel, tea.Cmd)` from Update.
- **SharedState**: Passed by pointer to tabs. Contains Docker client, K8s client, GitLab client, StreamManager, config. Only backend state is shared; UI state is per-tab.
- **GitLab auth discovery**: config → `GITLAB_TOKEN` env → `~/.config/glab-cli/config.yml` (handles `!!null` YAML tag). Project auto-detected from `git remote get-url origin`.
- **Multi-user tracking**: GitLab tabs query for both the authenticated user and configured `additional_users` (e.g. bot accounts).
- **Message types**: All in `pkg/messages/` to avoid circular dependencies between UI and backend packages. Exported message types are broadcast to all tabs; unexported (tab-local) types are only routed to the active tab.
- **Tab activation**: Root sends `messages.TabActivatedMsg` when switching tabs. Tabs that need deferred work (e.g. auto-selecting first item after list loads) set a `needsAutoSelect` flag in the broadcast list handler and act on it in the `TabActivatedMsg` handler — never return commands producing local message types from broadcast handlers, as the results will be lost if the tab isn't active.
- **Pane switching**: `Ctrl+W W` and `Alt+W` toggle focus between sidebar and log pane (vim-style). `Enter` on sidebar item also moves focus to logs.
- **Two-key sequences**: `gg` (go to top) and `Ctrl+W w` use pending state flags (e.g., `pendingG`, `pendingCtrlW`).

## Rules

- **Never commit personal info**: no names, emails, IP addresses, tokens, or company references
- **Solarized Light**: Test that text is readable on light background
- **Keep it simple**: minimal dependencies, no over-engineering
- **Keep code consistent across tabs**: All tabs (Docker, K8s, Issues, MRs, Pipelines) must follow the same patterns:
  - Tab struct fields: `client`, `sidebar`, `detailPane`/`logView`, `modal`, `focusSidebar`, `width`, `height`, `selectedIID`/`selectedID`, `notification`, `pendingCtrlW`, `refreshS`, `fetchSeq`, `pendingFetch`, `needsAutoSelect`
  - `Init()` returns `tea.Batch(fetchData(), tickRefresh())`
  - `Update()` handles: list msg → populate sidebar + set `needsAutoSelect`; `TabActivatedMsg` → auto-select; detail result msg → set detail pane; action msg → show notification + re-fetch; refresh tick → re-fetch + re-tick
  - Sidebar actions: `Enter` select, `o` open browser, `s` close/reopen toggle, `c` comment via `$EDITOR`
  - Pane switching: `Ctrl+W W` / `Alt+W` via `pendingCtrlW` flag and `toggleFocus()`
  - Detail fetch debounce: 150ms tick with `pendingFetch` + `fetchSeq` for stale response discard
  - Destructive actions use confirmation modal (`modal.Show` with callback)
  - Sidebar groups: "My X" → "Other X" → "All X" pattern
  - When adding a feature to one tab, check if it applies to other tabs and add it there too

## Conventions

- Do not add Co-Authored-By lines to commit messages
- Docker containers are grouped by `com.docker.compose.project` label; standalone containers go to "standalone" group
- Keybindings support both vim-style (hjkl) and arrow keys simultaneously via `key.NewBinding` with multiple keys
- Config path: `~/.config/lazydev/config.yaml` (XDG compliant)
- Sidebar width is 15% of terminal width (GitLab tabs use 25%)
- Log lines are truncated to pane width (no wrap by default); `w` toggles wrap mode
- Markdown content in GitLab detail panes is rendered via glamour with `WithWordWrap(paneWidth)`
- Relative GitLab URLs are resolved to absolute URLs; `/uploads/` paths use `/-/project/{id}/uploads/` format

## Current Status

All 8 phases complete, plus UX polish:

- Phase 1: Docker tab with live log tailing, sidebar, search, container actions
- Phase 2: Collapsible groups, confirmation modal, inspect detail pane
- Phase 3: Kubernetes tab with pod management, describe, YAML
- Phase 4: Log level filtering (f key), search highlighting, All Logs merged tab
- Phase 5: Dashboard tab with sortable table, Docker stats (CPU/mem)
- Phase 6: Exec shell (x), port-forward (p), scale deployment (S)
- Phase 7: Help overlay (?), command palette (:), goreleaser config
- UX polish: Solarized Light theme, vim gg/G navigation, sidebar `/` search, Ctrl+W W pane switching, cursor highlight in log pane, wrap toggle (w), log export (y/Y/e/E/o), Docker header & ANSI stripping, mouse click/scroll support, 1-9 tab selection
- Phase 8: GitLab integration — Issues (sprint grouping, related MRs), MRs (approve, merge, DiffviewOpen review), Pipelines (jobs, live job logs, retry/cancel), markdown rendering (glamour solarized), Ctrl+click URLs, tab auto-select on activation
