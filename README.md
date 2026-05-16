# lazydev

A terminal cockpit for working GitLab issues and merge requests with Claude Code as a pair programmer. Reads from a local SQLite mirror so the UI is instant, ships a query DSL on `/`, multi-select export, and one-key handoff (`C` / `P`) to a Claude Code session in a tmux split.

Built with Go and Bubble Tea v2. Solarized Light by default.

## Features

- **Three tabs**: Issues, MRs, Claude (active sessions).
- **Local SQLite cache** — every read renders from `~/.local/state/lazydev/cache.db` first; a background Syncer keeps it fresh. No network wait on startup, works offline for browsing. A sync indicator on the right of the status bar shows `starting…` / `prefetching N…` / `synced 5s ago` / `offline: <err>` in real time.
- **GraphQL bulk sync** — one paginated `namespace.workItems` query per 50 issues returns title/state + every widget (Status, Parent, Children, Linked items, Description). A 200-item prefetch is ~4 requests instead of ~200.
- **Rich detail pane** — `gh issue view`-style header strip (State, Status, Assignees, Labels, Parent, Milestone, Iteration, Author, Created, Updated, URL) + glamour-rendered body + footer sections for Related MRs, Child items, and Linked items (grouped `Blocked by` / `Blocks` / `Relates to`). Every `#NNN` / `!NNN` reference and URL is an OSC 8 hyperlink — `Ctrl+click` opens it in the browser.
- **Query DSL on `/`** — `assignee:@me`, `assignee:@ai`, `label:bug`, `state:open`, `kind:mr`, `updated:>7d`, plus bare fuzzy terms over title/body/notes (FTS5). Tokens are AND'd; quoted strings preserved. `Enter` commits the filter without dropping it.
- **Multi-select** — `Space` to mark, `v` for visual range, `Esc` to clear. All export and Claude dispatch keys act on the marked set (or the cursor item if nothing is marked).
- **Claude Code handoff** — `C` opens an interactive Claude session in a tmux window (or new tmux session if outside tmux); `P` spawns `claude -p` in the background and tees output to `.lazydev/claude-runs/<id>.log`. Both return immediately, persist a session record, and compose a structured prompt from the marked items. Interactive dispatch goes through a `/bin/sh` launcher script so non-POSIX shells (fish, nushell) work as the user's `$SHELL`.
- **Sessions tab** — Claude tab lists dispatched sessions from `.lazydev/sessions.json`; `Enter` re-attaches, `o` opens the originating issue/MR, `L` opens the run log, `d` drops the record.
- **AI handoff helpers** — `T` toggles assignee between self and the configured `ai_user`; `N` quick-creates an issue from a `$EDITOR` template assigned to `ai_user`.
- **Multi-assignee aware** — items with multiple assignees are tracked correctly across the sidebar grouping, AI-toggle, and export bundles.
- **Context export** — `y` copies marked items to clipboard via OSC52, `Y` writes to `/tmp/lazydev-ctx-*.md`, `X` pipes through `llm_command` (default `claude -p`). Default format is Claude-XML; markdown also supported.
- **Markdown rendering** — issue/MR descriptions and comments rendered with glamour (solarized).
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

## Workflow

lazydev is the **triage + handoff** surface; Claude Code is the **execution** surface. The intended loop:

1. **Triage** in the Issues / MRs tab. Sidebar paints instantly from cache; syncer refreshes in the background.
2. **Narrow** with `/` (query DSL): `assignee:@me` for your work, `assignee:@ai` for what's queued for Claude, `label:bug state:open updated:>3d` for recent bugs, etc.
3. **Pick** the item under the cursor — or `Space` to mark multiple and bundle them into one prompt.
4. **Dispatch**:
   - `C` — interactive Claude in a tmux split. Pair with it.
   - `P` — one-shot Claude that runs in the background; output streams to `.lazydev/claude-runs/<id>.log`.
5. **Track** in the **Claude tab**: every dispatched session is listed with its ref (`#421`, `!1024`), mode, and status. `Enter` re-attaches an interactive session; `L` opens the run log of a one-shot.

### The AI-queue convention

Set `gitlab.ai_user` to a bot account (e.g. `claude-bot`) and lazydev gives you bidirectional handoff with that user:

- `T` toggles an item's assignee between you and `ai_user` — push work to Claude, or take it back.
- `N` quick-creates an issue from a `$EDITOR` template auto-assigned to `ai_user`.
- `@ai` in the query DSL resolves to that username — `/assignee:@ai state:open` is Claude's open queue.

GitLab becomes the queue manager, the bot account is the queue holder, lazydev gives you one-key triage between you and Claude. No new infrastructure required.

## Keybindings

### Global

| Key            | Action              |
| -------------- | ------------------- |
| `q` / `Ctrl+C` | Quit                |
| `Tab`          | Next tab            |
| `Shift+Tab`    | Previous tab        |
| `1`–`9`        | Switch tab by index |
| `?`            | Help overlay        |
| `:`            | Command palette     |

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

### Query (sidebar)

| Key | Action                                           |
| --- | ------------------------------------------------ |
| `/` | Open query line (DSL)                            |
| `r` | Refresh now (nudges syncer + reloads from cache) |

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

cache:
  db_path: "" # default: $XDG_STATE_HOME/lazydev/cache.db
  sync_interval_s: 60 # incremental sync cadence
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

## Architecture

```
cmd/lazydev/main.go         entry: builds SharedState, wires syncer events into Bubble Tea
internal/
  app/                      SharedState: GitLab client, cache, syncer, Claude env/store
  cache/                    SQLite mirror (modernc.org/sqlite) + FTS5 search + Syncer goroutine
                            schema v3: issues / mrs / notes / related_mrs / linked_items / child_items / search_fts
  claude/                   Claude Code: discovery, structured prompt, sessions store, dispatch
  config/                   YAML config + defaults
  export/                   OSC52 clipboard, file write, Claude-XML / markdown builders
  gitlab/                   GitLab client + issues/MRs/notes
    workitems_graphql.go    paginated GraphQL bulk fetch for issues + widgets (status, parent, children, linked)
    sync.go                 ListMRsUpdatedAfter (REST) — MR sync is still REST since work-items don't apply
  query/                    Query DSL parser (`assignee:@me label:bug state:open`)
  ui/
    root.go                 RootModel, tab dispatch, command palette, sync indicator
    theme/                  Lip Gloss styles + keybindings
    components/             Sidebar (multi-select), DetailPane (OSC 8 + Ctrl+click), QueryLine, etc.
    tabs/                   IssuesTab, MRsTab, ClaudeTab + shared Options + dispatchClaude
pkg/messages/               Shared tea.Msg types (cross-package, avoids cycles)
```

## Cache hygiene

The Syncer auto-handles incremental updates, but if a schema mismatch ever appears or you suspect drift from items deleted on GitLab:

```bash
task wipe-cache   # rm ~/.local/state/lazydev/cache.db*  → next run does a fresh prefetch
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
