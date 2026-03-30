# lazydev

A unified terminal UI for Docker, Kubernetes, and GitLab. View logs, monitor status, manage resources, track issues, review MRs, and watch pipelines — all in one tool.

Inspired by [lazydocker](https://github.com/jesseduffield/lazydocker), [k9s](https://github.com/derailed/k9s), and [Tilt](https://tilt.dev/).

## Features

- **Tab-based UI** — Docker, Kubernetes, All Logs, Dashboard, Issues, MRs, Pipelines
- **Live log tailing** — real-time streaming with batched delivery for smooth rendering
- **Log search & filter** — `/` to search logs, `f` to cycle log level filter (ERROR+, WARN+, INFO+, DEBUG+)
- **Log level highlighting** — ERROR (red), WARN (yellow), INFO (green), DEBUG (cyan), FATAL (magenta bold)
- **Cursor navigation** — cursor highlight on current log line, `gg`/`G` for top/bottom
- **Wrap toggle** — `w` to toggle line wrapping in log pane
- **Log export** — `y` yank line, `Y` yank all, `e` export to file, `E` export JSON, `o` open in `$EDITOR`
- **Grouped resources** — Docker containers grouped by Compose project, K8s pods by namespace
- **Collapsible groups** — `Enter` to expand/collapse resource groups in sidebar
- **Auto-detect backends** — automatically finds Docker daemon, kubeconfig, and GitLab token (from glab CLI)
- **GitLab Issues** — view assigned/created issues, close/reopen, comment, assign
- **GitLab MRs** — track MRs, approve, merge, review with neovim DiffviewOpen
- **GitLab Pipelines** — view pipeline jobs, job logs, retry/cancel pipelines
- **Multi-user tracking** — track your own + bot account activity across issues/MRs/pipelines
- **Container actions** — restart, stop, remove, inspect, exec, port-forward, scale
- **Vim + arrow key navigation** — hjkl and arrow keys, `Ctrl+W W` / `Alt+W` for pane switching
- **Sidebar search** — `/` in sidebar for live resource filtering
- **Mouse support** — click to select tabs, resources, and log lines; scroll wheel in log pane
- **Solarized Light theme** — clean, readable color palette
- **LLM-friendly exports** — structured text/JSON export for AI-assisted debugging

## Installation

### From source

```bash
git clone https://github.com/abhishekrana/lazydev.git
cd lazydev
./bootstrap.sh   # install Taskfile runner
task init         # install dev tools
task build        # build binary
```

### Go install

```bash
go install github.com/abhishekrana/lazydev/cmd/lazydev@latest
```

## Usage

```bash
# Run with auto-detection (finds Docker and/or K8s automatically)
lazydev

# Specify Docker host
lazydev --docker-host tcp://localhost:2375

# Specify kubeconfig
lazydev --kubeconfig ~/.kube/my-config
```

## Keybindings

### Global

| Key            | Action                  |
| -------------- | ----------------------- |
| `q` / `Ctrl+C` | Quit                    |
| `1`-`4`        | Switch to tab by number |
| `Tab`          | Next tab                |
| `Shift+Tab`    | Previous tab            |
| `?`            | Help                    |

### Navigation

| Key                  | Action                              |
| -------------------- | ----------------------------------- |
| `j` / `Down`         | Move down                           |
| `k` / `Up`           | Move up                             |
| `Ctrl+W W` / `Alt+W` | Toggle pane focus (sidebar ↔ logs) |
| `Enter`              | Select item / move focus to logs    |
| `Esc`                | Back / cancel                       |
| `gg`                 | Go to top                           |
| `G`                  | Go to bottom (auto-follow)          |

### Log View

| Key | Action                                                         |
| --- | -------------------------------------------------------------- |
| `/` | Search logs                                                    |
| `f` | Cycle log level filter (ALL → ERROR+ → WARN+ → INFO+ → DEBUG+) |
| `w` | Toggle line wrapping                                           |
| `y` | Yank current line to clipboard (OSC52)                         |
| `Y` | Yank all filtered lines to clipboard                           |
| `e` | Export filtered logs to text file (`/tmp/`)                    |
| `E` | Export filtered logs to JSON file                              |
| `o` | Open filtered logs in `$EDITOR` at cursor line                 |

### Docker/K8s Actions

| Key | Action                     |
| --- | -------------------------- |
| `r` | Restart container/pod      |
| `s` | Stop container/pod         |
| `d` | Delete (with confirmation) |
| `D` | Describe / inspect         |
| `x` | Exec shell                 |
| `p` | Port forward               |
| `S` | Scale deployment           |

### GitLab Issues

| Key | Action                          |
| --- | ------------------------------- |
| `s` | Close/reopen issue              |
| `c` | Comment (opens `$EDITOR`)       |
| `a` | Assign to self                  |
| `o` | Open in browser                 |

### GitLab MRs

| Key | Action                                        |
| --- | --------------------------------------------- |
| `r` | Review in neovim (DiffviewOpen)               |
| `m` | Merge (with confirmation)                     |
| `A` | Approve                                       |
| `s` | Close/reopen                                  |
| `c` | Comment (opens `$EDITOR`)                     |
| `o` | Open in browser                               |

### GitLab Pipelines

| Key | Action                   |
| --- | ------------------------ |
| `R` | Retry failed pipeline    |
| `C` | Cancel running pipeline  |
| `o` | Open in browser          |

## UI Layout

```
┌─[Docker]──[Kubernetes]──[All Logs]──[Dashboard]──[Issues]──[MRs]──[Pipelines]─┐
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

Config file: `~/.config/lazydev/config.yaml`

```yaml
docker:
  host: "" # auto-detect if empty
  compose_detection: true # group by docker-compose project

kubernetes:
  kubeconfig: "" # auto-detect if empty
  context: "" # use current context if empty
  namespaces: [] # watch all if empty

gitlab:
  url: "" # auto-detect from glab CLI config
  token: "" # auto-detect from glab CLI or GITLAB_TOKEN env
  project: "" # auto-detect from git remote origin
  additional_users: [] # extra usernames to track (e.g. bot accounts)
  refresh_interval_s: 30

ui:
  theme: dark
  sidebar_width: 15 # percentage of terminal width
  log_buffer_size: 10000 # lines per source
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
- **[GitLab Go SDK](https://gitlab.com/gitlab-org/api/client-go)** for GitLab API

## Architecture

```
cmd/lazydev/main.go           Entry point
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
  gitlab/                     GitLab client, issues, merge requests, pipelines
  log/                        StreamManager, RingBuffer, filter, highlight
  export/                     Log export (text, JSON, file, OSC52 clipboard)
  config/                     YAML config loading
  discovery/                  Auto-detect backends
pkg/messages/                 Shared tea.Msg types
```

## License

MIT
