# Claude Code integration

GitLab-only cockpit, Claude Code as the only supported agent. This branch
delivers the first slice from `docs/research-ai-first-dev.md` §7 — the
unified human + Claude Code surface on top of the existing Issues/MRs
tabs.

## Goals (this branch)

1. Dispatch Claude Code against the current selection (issue or MR) with
   one keystroke, using the same structured context bundle `y/Y/X`
   already produce.
2. Persist sessions to `.lazydev/sessions.json` in the working tree so
   they survive restarts and can be re-attached.
3. Provide a minimal Sessions tab to list, attach, and clean up sessions.
4. Degrade gracefully when `claude` or `tmux` are absent.

## Non-goals (defer to follow-ups)

- `CLAUDE.md` viewer / editor tab.
- Auto draft-MR creation from a finished session (`M` key).
- Sub-agent palette (`.claude/agents/` awareness).
- Pipeline-failure → Claude triage.
- Context-budget HUD.

## Architecture

```
internal/claude/
  discovery.go   // find `claude` and `tmux` binaries; report missing
  prompt.go      // structured task bundle: reuses export.BuildClaudeXML
                 // + adds an instruction header per dispatch mode
  session.go     // Session struct, .lazydev/sessions.json read/write
                 // with file lock (advisory) to avoid concurrent corruption
  dispatch.go    // one-shot:   claude -p <prompt>          (foreground exec)
                 // interactive: tmux split-window in current pane,
                 //              new window if not in tmux, with prompt
                 //              piped into stdin or written to a tempfile
                 //              that claude reads on startup
```

- **Foreground tmux only.** No background daemon. If the user isn't in
  tmux, we shell out `tmux new-session -d -s lazydev-claude-<n>` and
  print the attach command in the notification bar. Sessions remain
  attachable after lazydev quits.
- **Sessions storage is per-repo.** `.lazydev/sessions.json` lives at
  the git working-tree root (discovered via `git rev-parse --show-toplevel`).
  Falls back to `$PWD` if not in a git repo.
- **Session ↔ issue mapping is explicit.** Each dispatch writes a record:
  `{id, kind, ref, branch, tmux_target, mode, prompt_path, created_at, last_seen_at}`.
- **Both dispatch modes supported.**
  - `P` — one-shot: `claude -p <context+instruction>` runs to completion
    in a tea.ExecProcess. Stdout captured to `.lazydev/claude-runs/<id>.log`.
    Notification when done.
  - `C` — interactive: structured context written to a tempfile, tmux
    split spawned running `claude` with that file argv'd in. User
    detaches with their usual tmux prefix; record stays in sessions.json.

## Config additions

New top-level `claude` block in `~/.config/lazydev/config.yaml`:

```yaml
claude:
  binary: claude                 # CLI binary name (looked up in PATH)
  spec_dir: docs/specs           # repo-relative spec directory
  prompts_dir: .lazydev/prompts  # repo-relative prompt-template dir
  session_file: .lazydev/sessions.json
  tmux_session: lazydev-claude   # name of tmux session created when
                                 # lazydev was started outside tmux
```

All fields have sensible defaults — the user does not need to set any of
this for the feature to work; `defaults.go` populates them.

## Keybindings (Issues + MRs)

| Key | Action |
|---|---|
| `C` | Interactive Claude Code session: build context → tmux split → `claude <tempfile>` |
| `P` | One-shot `claude -p`: build context → foreground exec → log result |

Both keys work on the cursor item *or* the multi-select mark set (same
semantics as `y/Y/X`). Both require the project's `claude` binary to be
resolvable; otherwise a notification explains what's missing.

## Sessions tab

A new tab "Claude" (added after MRs) lists sessions from the JSON file:

```
ID         Kind     Ref      Mode    Last seen     Status
abc123     issue    #421     int     2m ago        attached
def456     mr       !1024    one     5m ago        done
```

Keybindings on a row:
- `Enter` — `tmux attach -t <target>` (interactive only)
- `o` — open the originating issue/MR in the browser
- `d` — drop the session record (does not kill the tmux window — leaves
  that to the user via tmux itself)
- `r` — reload `.lazydev/sessions.json`

## Discovery & graceful degradation

`internal/claude/discovery.go` checks `exec.LookPath("claude")` and
`exec.LookPath("tmux")` once at app start. Results are stored on a
`claude.Env` value passed through `tabs.Options`. If `claude` is
missing:

- `C` / `P` show `"claude not in PATH (install: https://docs.anthropic.com/claude-code)"`.
- Sessions tab still loads; it just shows a banner.

If `tmux` is missing, `C` falls back to foreground exec (same as `P`)
with a one-time notification explaining the downgrade.

## Out-of-scope notes

- No abstraction over models, providers, or agents. Hard-code Claude Code
  conventions. The only adjustable knob is the binary name.
- We do NOT manage `~/.claude/projects/<encoded>/` — Claude Code owns
  that. We only track our dispatches; Claude's own resume mechanics
  remain authoritative.
- We do NOT parse Claude Code stdout for session IDs (format would
  couple us to a CLI surface that changes). Session identity is the
  random ID lazydev mints + the tmux target name.

## Implementation order

1. `internal/claude/` skeleton + tests for sessions JSON round-trip.
2. Config struct + defaults + wire through `app.SharedState` and
   `tabs.Options`.
3. `C`/`P` keybindings on Issues tab (reusing `buildExportItems`).
4. Mirror to MRs tab.
5. Sessions tab.
6. `task format && go build ./... && task lint`.
