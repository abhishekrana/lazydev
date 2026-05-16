---
name: lazydev
description: Query the local GitLab issues/MRs knowledge base maintained by the lazydev TUI. Use whenever the user references a ticket or MR by IID, asks about the status/discussion on a GitLab item, asks "what tickets are open about X", or wants to find work related to the code in front of you — anything that would otherwise require opening GitLab in a browser. The cache is read-only from your perspective and is kept fresh by the user running lazydev; do not write to it and do not call the GitLab API.
---

# lazydev: local GitLab knowledge base

The user runs `lazydev`, a TUI that mirrors GitLab issues and merge requests into a local SQLite cache at `~/.local/state/lazydev/cache.db`. The `lazydev` binary exposes a read-only CLI surface over that cache so you can answer questions about tickets and MRs without opening GitLab.

## When to invoke

Reach for these commands when the user:

- References a ticket or MR by ID: "issue #482", "MR !913", "look at #1234".
- Asks about status, assignees, or discussion: "what's the status of #N?", "who's reviewing !N?", "did anyone respond on #N?".
- Asks scope/discovery questions: "what tickets are open about auth?", "are there any MRs touching the cache layer?", "what's blocking #200?".
- Wants context before changing code: "what work has touched this module recently?" → `lazydev search <module-name>`.

If the user is asking about file/code state, prefer the usual tools (Read, Grep). Use `lazydev` for GitLab metadata.

## Commands

All commands return JSON. Default list output is NDJSON (one object per line) so you can pipe through `jq -c` or `head`. Pass `--pretty` for an indented JSON array, which is easier to read when you intend to use the result directly.

### `lazydev search <query> [--limit N] [--pretty]`

FTS5 search across issues and MRs (title + body + notes). Returns a JSON array sorted by relevance.

```
[{"kind":"issue|mr","iid":123,"title":"…","snippet":"…","score":-3.1}, …]
```

### `lazydev issue list [--query "DSL"] [--limit N] [--pretty]`

### `lazydev mr   list [--query "DSL"] [--limit N] [--pretty]`

Filtered list of issues / MRs. Without `--query` you get the most-recently-updated open items. Output:

```
{"iid":…,"title":…,"state":…,"assignees":[…],"labels":[…],"updated_at":…,"web_url":…}
```

### `lazydev issue show <IID> [--with-notes] [--pretty]`

Single issue, with linked items and child items. Notes are omitted by default — pass `--with-notes` only when the discussion is relevant to the question.

```
{"issue":{…},"notes":[…],"related_mrs":[…],"linked_items":[…],"child_items":[…]}
```

### `lazydev mr show <IID> [--with-notes] [--pretty]`

Single MR.

```
{"mr":{…},"notes":[…]}
```

## Query DSL

The `--query` flag accepts the same expression language the TUI uses. Tokens are space-separated and AND'd together. Quote strings that contain spaces.

| Token                    | Meaning                                            |
| ------------------------ | -------------------------------------------------- |
| `state:open`             | Default — also `closed`, `merged` (MR-only), `all` |
| `assignee:@me`           | The authenticated GitLab user                      |
| `assignee:@ai`           | The configured AI-handoff user                     |
| `assignee:@none`         | Unassigned items                                   |
| `assignee:@any`          | No assignee filter (default)                       |
| `assignee:<username>`    | Specific user                                      |
| `author:<username>`      | Filter by author                                   |
| `label:foo`              | Has the `foo` label (repeat for AND)               |
| `kind:issue` / `kind:mr` | Restrict cross-kind queries                        |
| `updated:>7d`            | Updated in the last 7 days (also `h`/`m`/`s`)      |
| `updated:<30d`           | Last update was more than 30 days ago              |
| `updated:>2026-01-01`    | Absolute date                                      |
| bare terms               | Fuzzy match on title + description                 |

Examples:

```
lazydev issue list --query 'assignee:@me state:open' --pretty
lazydev mr list --query 'state:open label:area:auth updated:>14d'
lazydev search 'rate limit'
```

## Reading the output

- For lists, prefer `--pretty` when you'll consume the data yourself. Stay on NDJSON when piping through `jq -c` or counting lines.
- IIDs are integers; the `web_url` field is the canonical GitLab link to surface in your reply.
- Timestamps are ISO-8601 UTC.
- An empty `assignees` array means unassigned; a missing field means the data wasn't available.

## Don'ts

- Don't open `cache.db` directly with `sqlite3` — the schema can change between lazydev releases.
- Don't hit the GitLab API yourself. The TUI keeps the cache fresh; if the user reports stale data, ask them to run `lazydev` and let the syncer catch up rather than fetching live.
- Don't write to the cache. All `lazydev` subcommands listed here are read-only.
- If a command errors with "cache is empty" or "cache not found", tell the user to run `lazydev` once so the syncer can prefetch from GitLab.
