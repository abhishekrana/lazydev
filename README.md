# lazydk

A unified terminal UI for Docker and Kubernetes. View logs, monitor status, and manage resources across both platforms in one tool.

Inspired by [lazydocker](https://github.com/jesseduffield/lazydocker), [k9s](https://github.com/derailed/k9s), and [Tilt](https://tilt.dev/).

## Features

- **Tab-based UI** — separate tabs for Docker, Kubernetes, All Logs, and Dashboard
- **Live log tailing** — real-time streaming with batched delivery for smooth rendering
- **Log search** — press `/` to filter logs by text or regex
- **Log level highlighting** — ERROR (red), WARN (yellow), INFO (green), DEBUG (blue)
- **Grouped resources** — Docker containers grouped by Compose project, K8s pods by namespace
- **Auto-detect backends** — automatically finds Docker daemon and kubeconfig
- **Container actions** — restart, stop, remove, inspect, exec, port-forward, scale
- **Vim + arrow key navigation** — hjkl and arrow keys both work

## Installation

### From source

```bash
git clone https://github.com/abhishek-rana/lazydk.git
cd lazydk
make build
```

### Go install

```bash
go install github.com/abhishek-rana/lazydk/cmd/lazydk@latest
```

## Usage

```bash
# Run with auto-detection (finds Docker and/or K8s automatically)
lazydk

# Specify Docker host
lazydk --docker-host tcp://localhost:2375

# Specify kubeconfig
lazydk --kubeconfig ~/.kube/my-config
```

## Keybindings

### Global

| Key | Action |
|-----|--------|
| `q` / `Ctrl+C` | Quit |
| `Tab` | Next tab |
| `Shift+Tab` | Previous tab |
| `?` | Help |

### Navigation

| Key | Action |
|-----|--------|
| `j` / `Down` | Move down |
| `k` / `Up` | Move up |
| `h` / `Left` | Focus sidebar |
| `l` / `Right` | Focus log pane |
| `Enter` | Select / confirm |
| `Esc` | Back / cancel |
| `g` | Scroll to top |
| `G` | Scroll to bottom (auto-follow) |

### Actions

| Key | Action |
|-----|--------|
| `/` | Search logs |
| `f` | Filter by log level |
| `r` | Restart container/pod |
| `s` | Stop container/pod |
| `d` | Delete |
| `D` | Describe / inspect |
| `y` | View YAML |
| `x` | Exec shell |
| `p` | Port forward |
| `S` | Scale |

## UI Layout

```
┌─[Docker]──[Kubernetes]──[All Logs]──[Dashboard]──────┐
│ ┌──────────────┬────────────────────────────────────┐ │
│ │ Resources    │ Logs / Details                     │ │
│ │              │                                    │ │
│ │ ▼ my-app     │ 10:15:32 INFO  Server started      │ │
│ │   postgres ● │ 10:15:33 WARN  Slow query: 2.3s   │ │
│ │   redis    ● │ 10:15:34 ERROR Connection refused  │ │
│ │   api      ● │ 10:15:35 INFO  Retrying...         │ │
│ │              │                                    │ │
│ │ ▼ monitoring │                                    │ │
│ │   prometheus │                                    │ │
│ └──────────────┴────────────────────────────────────┘ │
│ [q]uit [/]search [:]cmd [?]help    ctx: minikube     │
└──────────────────────────────────────────────────────┘
```

## Configuration

Config file: `~/.config/lazydk/config.yaml`

```yaml
docker:
  host: ""                    # auto-detect if empty
  compose_detection: true     # group by docker-compose project

kubernetes:
  kubeconfig: ""              # auto-detect if empty
  context: ""                 # use current context if empty
  namespaces: []              # watch all if empty

ui:
  theme: dark
  sidebar_width: 30           # percentage of terminal width
  log_buffer_size: 10000      # lines per source
  timestamps: true
  wrap_lines: false
  refresh_interval_s: 5
```

## Tech Stack

- **Go** with [Bubble Tea v2](https://github.com/charmbracelet/bubbletea) (Elm architecture)
- **[Lip Gloss v2](https://github.com/charmbracelet/lipgloss)** for styling
- **[Bubbles v2](https://github.com/charmbracelet/bubbles)** for components
- **[Docker SDK](https://pkg.go.dev/github.com/docker/docker/client)** for Docker interaction
- **[client-go](https://github.com/kubernetes/client-go)** for Kubernetes interaction

## Architecture

```
cmd/lazydk/main.go           Entry point
internal/
  app/                        SharedState, backend wiring
  ui/
    root.go                   Root Bubble Tea model, tab dispatch
    theme/                    Lip Gloss styles, keybindings
    components/               Reusable widgets (sidebar, logview, tabbar, statusbar)
    tabs/                     Tab models (docker, kubernetes, logs, dashboard)
    layout/                   Split pane primitives
  docker/                     Docker client, containers, compose, actions
  kube/                       K8s client, pods, deployments, services, events
  log/                        StreamManager, RingBuffer, filter, highlight
  config/                     YAML config loading
  discovery/                  Auto-detect backends
pkg/messages/                 Shared tea.Msg types
```

## License

MIT
