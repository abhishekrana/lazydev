# lazydev

A terminal cockpit for working GitLab issues and merge requests with Claude Code as a pair programmer. Reads from a local SQLite mirror so the UI is instant, ships a query DSL on `/`, multi-select export, saved views, and one-key handoff (`C` / `P`) to a Claude Code session in a tmux split.

Built with Go and Bubble Tea v2. Solarized Light by default.

## Features

- **Three tabs**: Issues, MRs, Claude (active sessions).
- **Local SQLite cache** — every read renders from `~/.local/state/lazydev/cache.db` first; a background Syncer keeps it fresh. No network wait on startup, works offline for browsing.
- **Query DSL on `/`** — `assignee:@me`, `assignee:@ai`, `label:bug`, `state:open`, `kind:mr`, `updated:>7d`, plus bare fuzzy terms over title/body/notes (FTS5). Tokens are AND'd; quoted strings preserved.
- **Saved views** — number keys `1`–`9` recall views from `~/.config/lazydev/views.yaml`. Defaults seeded on first run: `mine`, `ai-queue`, `review`, `recent`. Manage via `:save <name> <expr>`, `:view <name>`, `:del <name>`.
- **Multi-select** — `Space` to mark, `v` for visual range, `Esc` to clear. All export and Claude dispatch keys act on the marked set (or the cursor item if nothing is marked).
- **Claude Code handoff** — `C` opens an interactive Claude session in a tmux window (or new tmux session if outside tmux); `P` runs `claude -p` one-shot and tees output to `.lazydev/claude-runs/<id>.log`. Both compose a structured prompt from the marked items.
- **Sessions tab** — Claude tab lists dispatched sessions from `.lazydev/sessions.json`; `Enter` re-attaches, `o` opens the originating issue/MR, `L` opens the run log, `d` drops the record.
- **AI handoff helpers** — `T` toggles assignee between self and the configured `ai_user`; `N` quick-creates an issue from a `$EDITOR` template assigned to `ai_user`.
- **Context export** — `y` copies marked items to clipboard via OSC52, `Y` writes to `/tmp/lazydev-ctx-*.md`, `X` pipes through `llm_command` (default `claude -p`). Default format is Claude-XML; markdown also supported.
- **Markdown rendering** — issue/MR descriptions and comments rendered with glamour (solarized). `Ctrl+click` opens URLs in the browser.
- **MR review** — `R` on an MR opens neovim with `DiffviewOpen` against the target branch.
- **Multi-user tracking** — track yourself plus `additional_users` (bot accounts) across issues and MRs.
- **Vim + arrow keys** — `hjkl`, `gg` / `G`, `Ctrl+W W` / `Alt+W` to switch pane focus.

## Installation

### From source

```bash
git clone https://github.com/abhishek-rana/lazydev.git
cd lazydev
./bootstrap.sh   # install Taskfile runner
task init        # install dev tools (goimports, golangci-lint)
task build       # build binary (output: ./lazydev)
task install     # install to ~/.local/bin
```

### Go install

```bash
go install github.com/abhishek-rana/lazydev/cmd/lazydev@latest
```

## Usage

```bash
# Auto-detects GitLab project from `git remote get-url origin`
cd ~/my-gitlab-project && lazydev
```

GitLab auth is auto-detected in this order: `gitlab.token` in config, `GITLAB_TOKEN` env, `~/.config/glab-cli/config.yml`. Lazydev refuses to start if GitLab is not configured.

Claude Code integration activates automatically when the `claude` binary is on `PATH`. `tmux` is required for the interactive (`C`) dispatch path; the one-shot path (`P`) works without it.

## Keybindings

### Global

| Key            | Action                                           |
| -------------- | ------------------------------------------------ |
| `q` / `Ctrl+C` | Quit                                             |
| `Tab`          | Next tab                                         |
| `Shift+Tab`    | Previous tab                                     |
| `1`–`9`        | Recall saved view (falls back to tab if no view) |
| `?`            | Help overlay                                     |
| `:`            | Command palette                                  |

### Navigation

| Key                  | Action                                 |
| -------------------- | -------------------------------------- |
| `j` / `↓`            | Move down                              |
| `k` / `↑`            | Move up                                |
| `gg`                 | Top                                    |
| `G`                  | Bottom                                 |
| `Enter`              | Select / collapse group / focus detail |
| `Ctrl+W W` / `Alt+W` | Toggle pane focus (sidebar ↔ detail)  |
| `Esc`                | Cancel / clear marks                   |

### Query & views (sidebar)

| Key                | Action                                           |
| ------------------ | ------------------------------------------------ |
| `/`                | Open query line (DSL)                            |
| `r`                | Refresh now (nudges syncer + reloads from cache) |
| `1`–`9`            | Recall saved view                                |
| `:save <n> <expr>` | Save current expression as a named view          |
| `:view <name>`     | Apply a saved view to the active tab             |
| `:del <name>`      | Delete a saved view                              |

Query DSL examples:

```
assignee:@me state:open           # my open work
assignee:@ai                      # Claude's queue
label:bug state:open updated:>7d  # recent bug reports
kind:mr author:@me                # my MRs across both tabs
refresh                           # bare fuzzy: matches title/body/notes
```

Variables: `@me` (authenticated user), `@ai` (`gitlab.ai_user`), `@none` (unassigned), `@any` (any).

### Select & export

| Key     | Action                                                        |
| ------- | ------------------------------------------------------------- |
| `Space` | Mark current item                                             |
| `v`     | Toggle visual range mode                                      |
| `Esc`   | Clear marks                                                   |
| `y`     | Copy marked → clipboard (OSC52, works over SSH/tmux)          |
| `Y`     | Write marked → `/tmp/lazydev-ctx-*.md`                        |
| `X`     | Pipe marked → `llm_command` (default `claude -p`)             |
| `C`     | Dispatch Claude Code **interactive** (tmux split)             |
| `P`     | Dispatch Claude Code **one-shot** (`claude -p`, logs to file) |

If nothing is marked, the cursor item is used.

### Issues tab

| Key     | Action                                                  |
| ------- | ------------------------------------------------------- |
| `Enter` | Open detail (focuses detail pane)                       |
| `o`     | Open in browser                                         |
| `s`     | Close / reopen (with confirmation)                      |
| `c`     | Comment via `$EDITOR`                                   |
| `a`     | Assign to self                                          |
| `T`     | Toggle assignee self ↔ `ai_user`                       |
| `N`     | Quick-create issue from template, assigned to `ai_user` |

### MRs tab

| Key     | Action                            |
| ------- | --------------------------------- |
| `Enter` | Open detail                       |
| `o`     | Open in browser                   |
| `R`     | Review in neovim (`DiffviewOpen`) |
| `m`     | Merge (with confirmation)         |
| `A`     | Approve (with confirmation)       |
| `s`     | Close / reopen                    |
| `c`     | Comment via `$EDITOR`             |
| `T`     | Toggle assignee self ↔ `ai_user` |

### Claude tab

| Key     | Action                                         |
| ------- | ---------------------------------------------- |
| `Enter` | Re-attach interactive session (tmux)           |
| `o`     | Open originating issue/MR in browser           |
| `L`     | Open run log (`.lazydev/claude-runs/<id>.log`) |
| `d`     | Drop session record                            |
| `r`     | Reload sessions from `.lazydev/sessions.json`  |

### Detail pane

| Key          | Action              |
| ------------ | ------------------- |
| `j` / `↓`    | Scroll down         |
| `k` / `↑`    | Scroll up           |
| `gg` / `G`   | Top / bottom        |
| `Ctrl+click` | Open URL in browser |

## Configuration

Config file: `~/.config/lazydev/config.yaml` (XDG compliant).

```yaml
gitlab:
  url: "" # auto-detect from glab CLI
  token: "" # auto-detect from glab CLI or GITLAB_TOKEN
  project: "" # auto-detect from `git remote get-url origin`
  additional_users: [] # extra usernames to track (e.g. bot accounts)
  ai_user: "" # the GitLab username `@ai` resolves to
  refresh_interval_s: 20

cache:
  db_path: "" # default: $XDG_STATE_HOME/lazydev/cache.db
  sync_interval_s: 20
  prefetch_window_days: 30 # initial backfill window

export:
  format: claude-xml # "claude-xml" | "markdown"
  llm_command: "claude -p" # invoked by X
  include_comments: true
  include_related_mrs: stub # "stub" | "full" | "none"

ui:
  theme: light
  sidebar_width: 30 # percent of terminal width
  wrap_lines: false

claude:
  binary: claude
  spec_dir: docs/specs
  prompts_dir: .lazydev/prompts
  session_file: .lazydev/sessions.json
  tmux_session: lazydev-claude
```

Views file: `~/.config/lazydev/views.yaml`. Created on first run with defaults (`mine`, `ai-queue`, `review`, `recent`).

## Architecture

```
cmd/lazydev/main.go     entry: builds SharedState, wires syncer events into Bubble Tea
internal/
  app/                  SharedState: GitLab client, cache, syncer, views, Claude env/store
  cache/                SQLite mirror (modernc.org/sqlite) + FTS5 search + Syncer goroutine
  claude/               Claude Code: discovery, structured prompt, sessions store, dispatch
  config/               YAML config + defaults
  export/               OSC52 clipboard, file write, Claude-XML / markdown builders
  gitlab/               GitLab client + issues/MRs/notes + updated_after sync helpers
  query/                Query DSL parser (`assignee:@me label:bug state:open`)
  views/                Saved-views YAML store (1–9 recall)
  ui/
    root.go             RootModel, tab dispatch, command palette, view application
    theme/              Lip Gloss styles + keybindings
    components/         Sidebar (multi-select), DetailPane, QueryLine, Modal, etc.
    tabs/               IssuesTab, MRsTab, ClaudeTab + shared Options + dispatchClaude
pkg/messages/           Shared tea.Msg types (cross-package, avoids cycles)
```

## Tech Stack

- **Go** with [Bubble Tea v2](https://github.com/charmbracelet/bubbletea)
- **[Lip Gloss v2](https://github.com/charmbracelet/lipgloss)** styling
- **[Bubbles v2](https://github.com/charmbracelet/bubbles)** components
- **[GitLab Go SDK](https://gitlab.com/gitlab-org/api/client-go)** for the GitLab API
- **[modernc.org/sqlite](https://gitlab.com/cznic/sqlite)** pure-Go SQLite (no CGo)
- **[Glamour v2](https://github.com/charmbracelet/glamour)** for terminal markdown
- **[Claude Code](https://docs.anthropic.com/claude-code)** for AI handoff (`C` / `P`)

## License

MIT
