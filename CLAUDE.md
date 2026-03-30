# CLAUDE.md

## Project Overview

**lazydk** is a unified TUI for Docker and Kubernetes — view logs, monitor status, and manage resources in one terminal tool. Built with Go and Bubble Tea v2.

## Build & Run

```bash
make build     # Build binary (output: ./lazydk)
make run       # Build and run
make clean     # Remove binary
make tidy      # go mod tidy
go build ./... # Build all packages (check compilation)
```

## Project Structure

- `cmd/lazydk/main.go` — Entry point, creates SharedState and root model
- `internal/app/app.go` — SharedState: holds Docker/K8s clients, StreamManager, config
- `internal/ui/root.go` — Root Bubble Tea model, handles tab switching and message dispatch
- `internal/ui/theme/` — All styles (Lip Gloss) and keybindings in one package to avoid import cycles
- `internal/ui/components/` — Reusable UI widgets: TabBar, Sidebar, LogView, StatusBar
- `internal/ui/tabs/` — Tab models: DockerTab, (KubernetesTab, LogsTab, DashboardTab planned)
- `internal/ui/layout/` — Split pane layout helpers
- `internal/docker/` — Docker SDK wrapper: client, container list/logs/inspect, actions, compose grouping
- `internal/kube/` — Kubernetes client-go wrapper (planned)
- `internal/log/` — Log subsystem: StreamManager (goroutine lifecycle), RingBuffer, filter, highlight
- `internal/config/` — YAML config struct and defaults
- `internal/discovery/` — Auto-detect Docker daemon and kubeconfig at startup
- `pkg/messages/` — All shared `tea.Msg` types used across packages

## Key Architecture Decisions

- **Bubble Tea v2 API**: `Init()` has no args, `View()` returns `tea.View` (not string), use `tea.KeyPressMsg` (not `tea.KeyMsg`), `AltScreen` is set on `tea.View` struct
- **Import cycle prevention**: Styles and keys live in `internal/ui/theme/` — components import `theme`, not `ui`
- **Log streaming**: Goroutines per source → fan-in channel → batched delivery (50ms/100 lines) → `LogBatchMsg` to Bubble Tea. Ring buffer bounds memory at 10k lines per source.
- **TabModel interface**: Defined in `internal/ui/root.go` — `Init()`, `Update()`, `View()`, `Title()`, `SetSize()`. Each tab returns `(ui.TabModel, tea.Cmd)` from Update.
- **SharedState**: Passed by pointer to tabs. Contains Docker client, K8s client, StreamManager, config. Only backend state is shared; UI state is per-tab.
- **Message types**: All in `pkg/messages/` to avoid circular dependencies between UI and backend packages.

## Conventions

- Do not add Co-Authored-By lines to commit messages
- Docker containers are grouped by `com.docker.compose.project` label; standalone containers go to "standalone" group
- Keybindings support both vim-style (hjkl) and arrow keys simultaneously via `key.NewBinding` with multiple keys
- Config path: `~/.config/lazydk/config.yaml` (XDG compliant)

## Current Status

- Phase 1 complete: Docker tab with live log tailing, sidebar, search, container actions
- Planned: Kubernetes tab, All Logs merged view, Dashboard tab, advanced actions (exec, port-forward, scale)
