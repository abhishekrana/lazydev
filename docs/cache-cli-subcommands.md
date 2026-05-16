# Expose lazydev cache as a Claude knowledge base — single-binary CLI + embedded skill

## Context

`lazydev` already maintains a fresh local mirror of GitLab issues/MRs in SQLite (~8 MB, WAL mode, FTS5 index). We want Claude Code to be able to use this mirror as a knowledge base during pair-programming — e.g. "show me the open tickets blocking #123", "summarize the discussion on !456", "find all MRs touching the auth middleware".

**Distribution constraint that drives the design**: lazydev ships as a standalone binary that other developers install. End users do not clone the repo. So Claude-facing docs cannot live in `.claude/skills/…` or `CLAUDE.md` checked into this repo — those don't travel with the binary. The docs have to be _inside_ the binary.

Researched alternatives (MCP server, Anthropic's reference SQLite MCP server, repo-side SKILL.md) and landed on **CLI subcommands with thorough `--help` + an opt-in `install-skill` command that writes an embedded SKILL.md to the user's home**. References:

- Skills vs MCP framing: [Verdent](https://www.verdent.ai/guides/claude-skills-vs-mcp), [Morph](https://www.morphllm.com/claude-code-skills-mcp-plugins).
- Shell-attached agents prefer CLIs over MCP: [Hutchison](https://allen.hutchison.org/2026/03/14/mcp-isnt-dead-you-just-arent-the-target-audience/).
- Anthropic's reference SQLite MCP server was archived with an unpatched SQL injection — don't reuse it.

A `lazydev mcp` subcommand stays deferred; the CLI shape designed here is the right substrate to wrap it later if needed.

## End-user experience

```
# install the binary (already works today)
task install          # → ~/.local/bin/lazydev

# use the TUI as before
lazydev

# new: query the cache from any shell / Claude session
lazydev search 'auth middleware'
lazydev issue show 123 --pretty
lazydev mr list --query 'state:open assignee:@me'

# opt-in better Claude integration (once per user)
lazydev install-skill        # writes ~/.claude/skills/lazydev/SKILL.md
```

Zero repo files required on the end-user machine.

## Scope

1. Add subcommand dispatch to `cmd/lazydev/main.go` (no-arg path keeps launching the TUI).
2. Implement five read-only subcommands emitting JSON / NDJSON:
   - `lazydev search <query>` — FTS5 across issues + MRs (uses `Store.Search`)
   - `lazydev issue list [--query "DSL"]` — filtered list (query DSL → `cache.Filter` → `Store.ListIssues`)
   - `lazydev mr list [--query "DSL"]`
   - `lazydev issue show <IID> [--with-notes]` — issue + linked items + child items + notes
   - `lazydev mr show <IID> [--with-notes]`
3. Add `lazydev install-skill` that writes an embedded `SKILL.md` to `~/.claude/skills/lazydev/SKILL.md` (with `--force` to overwrite, `--print` to dump to stdout instead).
4. Thorough `--help` per subcommand (also embedded — `flag.Usage`).

All five query subcommands are thin wrappers — no new business logic, no new SQL.

## Files to create / modify

### Modified

- **`cmd/lazydev/main.go`** — wrap current body in `runTUI()` and add `switch os.Args[1]` dispatcher. Stdlib `flag.NewFlagSet` per subcommand; no new dep.

### New (all under `cmd/lazydev/`)

- **`cli_query.go`** — DSL parsing → `cache.Filter` (calls existing `internal/query.Parse`, reuses the same `@me`/`@ai`/`@none` resolution as the TUI).
- **`cli_issues.go`** — `cmdIssueList`, `cmdIssueShow`.
- **`cli_mrs.go`** — `cmdMRList`, `cmdMRShow`.
- **`cli_search.go`** — `cmdSearch` over `Store.Search`.
- **`cli_output.go`** — JSON helpers: NDJSON for lists, single object for show, array for search. `--pretty` flag for human use.
- **`cli_skill.go`** — `cmdInstallSkill`. Uses `//go:embed skill.md` to bundle the skill content into the binary.
- **`cmd/lazydev/skill.md`** — the SKILL.md content (frontmatter + body). Lives in the repo source tree but gets compiled _into_ the binary via `embed`; not delivered as a file to end users.

### Reused (no changes)

- `internal/cache/store.go` — `Open`, `ListIssues`, `GetIssue`, `ListMRs`, `GetMR`, `ListLinkedItems`, `ListChildItems`, `Search`.
- `internal/cache/filter.go` — `Filter{State, Assignee, Author, Labels, Text, UpdatedAfter, UpdatedBefore, Limit}`.
- `internal/cache/search.go` — `SearchHit{Kind, IID, Title, Snippet, Score}`.
- `internal/query/parser.go` — `Parse(expression)` returns the same `Filter` the TUI consumes.
- `internal/config/` — `Load()` to find the cache path + AI user. (Not `app.NewSharedState` — that fails closed without GitLab config and isn't needed for read-only cache access.)
- `pkg/messages/messages.go` — `GitLabIssue`, `GitLabMR`, `GitLabNote`, `GitLabLinkedItem`, `GitLabChildItem`. Add JSON tags where missing.

### Key behavior

- CLI does **not** open a GitLab client or start the Syncer — pure cache reader. The TUI keeps the cache fresh.
- If the cache file is missing or empty, the CLI exits non-zero with `run lazydev once to populate the cache`. No silent zero results.
- SQLite WAL mode → "TUI writing + CLI reading" is safe and lock-free.

## JSON output contract (stable surface the embedded skill describes)

```
search:       [{"kind":"issue|mr","iid":123,"title":"…","snippet":"…","score":-3.1}, …]
issue list:   {"iid":…,"title":…,"state":…,"status":…,"assignees":[…],"labels":[…],"updated_at":…,"web_url":…}   (NDJSON)
mr list:      {"iid":…,"title":…,"state":…,"source_branch":…,"target_branch":…,"assignees":[…],"reviewers":[…],"labels":[…],"updated_at":…,"web_url":…}   (NDJSON)
issue show:   {"issue":{…},"notes":[…],"related_mrs":[…],"linked_items":[…],"child_items":[…]}
mr show:      {"mr":{…},"notes":[…]}
```

`--with-notes` defaults to `false` on show to keep responses small; skill instructs Claude to add it only when discussion is relevant.

## Embedded skill content (`cmd/lazydev/skill.md`)

Frontmatter:

```yaml
---
name: lazydev
description: Query the local GitLab issues/MRs knowledge base maintained by lazydev. Use when the user references a ticket/MR by IID, asks "what tickets are open about X", asks about discussion/status on a GitLab item, or wants to find related work — anything that would otherwise require opening GitLab.
---
```

Body covers:

1. **When to invoke** — trigger phrases ("issue #N", "MR !N", "what's blocking X", "find tickets about Y").
2. **Commands & flags** — verbatim copy of the JSON contract above.
3. **Query DSL primer** — `assignee:@me|@ai|@none|@any`, `label:foo`, `state:open|closed|merged|all`, `kind:issue|mr`, `updated:>7d`, plus bare fuzzy terms. AND-joined.
4. **Output reading hints** — NDJSON line-by-line for lists; `--pretty` only for humans.
5. **Don'ts** — don't open `cache.db` directly with `sqlite3`; don't hit GitLab over the network; don't write to the cache.

## Verification

1. **Build**: `task build && ./lazydev search 'auth' | head` — prints JSON hits, exit 0.
2. **DSL parity**: `./lazydev issue list --query 'assignee:@me state:open' --pretty` — output matches the TUI Issues tab under the same query.
3. **Show with notes**: `./lazydev issue show 123 --with-notes --pretty | jq '.notes | length'` — non-zero for a ticket with discussion.
4. **Cold cache hint**: `task wipe-cache && ./lazydev search foo` — non-zero exit with "run lazydev to populate cache"; no panic.
5. **Concurrent read**: TUI in pane A, `lazydev search bug` in pane B during a sync — no errors, consistent results.
6. **Skill install (clean)**: `rm -rf ~/.claude/skills/lazydev && lazydev install-skill && cat ~/.claude/skills/lazydev/SKILL.md | head` — file present, content matches embed.
7. **Skill install (exists, --force)**: re-run without `--force` → refuses; with `--force` → overwrites.
8. **Skill print**: `lazydev install-skill --print | head` — prints to stdout, does not write.
9. **End-to-end with Claude Code**: in a fresh Claude Code session, ask "status of issue #N" → Claude auto-invokes `lazydev issue show N --pretty` via Bash. Confirm in the tool-use trace.
10. **Single-artifact check**: `cp ./lazydev /tmp/elsewhere && cd /tmp && ./elsewhere search foo` — works without the repo present (proves no `.claude/skills/` or `CLAUDE.md` dependency at runtime).
11. **Lint / format / build clean**: `task check`.

## Implementation order

1. Subcommand dispatcher in `main.go` + `cli_output.go` helper.
2. `cli_search.go` (smallest verb, validates the JSON pipeline).
3. `cli_issues.go` list+show, then `cli_mrs.go`.
4. `skill.md` + `cli_skill.go` with `//go:embed`.
5. Manual run-through of the verification checklist.
6. Plan-docs followup: copy this plan into `docs/` on the feature branch and commit before code (per repo convention).
