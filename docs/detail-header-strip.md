# Detail-pane header strip rewrite

## Goal

Replace the current "one big formatted string scrolled as a flat list" detail layout with a `gh issue view`-style structure: compact **header strip** of key:value rows, **horizontal rule**, **markdown body**, **horizontal rule**, **footer sections** (related MRs, comments). Information density matches GitLab's right rail without the column cost — terminal-native, no third pane.

Applies to both Issues and MRs tabs. Out of scope: new GitLab data we don't already cache (weight, due date, parent, work-item status, multi-assignee, linked-item relation types, checklist items) — each gets its own follow-up plan.

## Current state

- `internal/gitlab/issues.go:FormatIssueDetail` and `internal/gitlab/mergerequests.go:FormatMRDetail` build a single `string` that the tab hands to `DetailPane.SetContent(title, content)`.
- `DetailPane` (`internal/ui/components/detailpane.go`) splits `content` on `\n`, renders the title row separately via `theme.ActiveTabStyle`, then scrolls `lines` underneath. `Ctrl+click` URL detection works per-line on the rendered content.
- Current header is six bare `key: value\n` lines (Assignee / Author / Labels / Milestone / Iteration / Created / Updated / URL) followed by a `─` rule, then "MERGE REQUESTS" block, then markdown via glamour, then "COMMENTS".

## Target layout

```
#2502  ● Open  ·  To do                                           ← pane.title (one line, truncated)
Phase 2: Action chunk support for client-server policy inference, using `step_chunk()`

Assignees   Burak Demirbilek, Jonathan Pirnay
Labels      710-demo, obj/ai-deployment
Milestone   —
Iteration   Animore iterations v1.2  (May 4 – 17, 2026)
Author      Jonathan Pirnay
Updated     2026-05-06 14:22
URL         https://gitlab.com/alphaignis/alpha/-/issues/2502
──────────────────────────────────────────────────────────
<glamour-rendered description>
──────────────────────────────────────────────────────────
Related MRs (3)
  ✓ !2490 Integrate AsyncChunkStepper into alpha_gym env API   merged
  ✓ !2489 alpha_gym: Rename 'RTCManager' to 'AsyncChunkStepper' merged
  ✓ !2498 Phase 1: Action chunk support for client-server …    merged
──────────────────────────────────────────────────────────
Comments (5)
  …
```

Notes on rendering:

- The title row stays the `DetailPane.title` (rendered once via the focused/unfocused header style, never scrolls). Format: `#<IID>  <state-glyph> <State>  ·  <Title>`, truncated to width with ellipsis.
- The second line (also fixed at the top of `content`, scrolls with body) repeats the full title for users who want to read it un-truncated. Skip if title fits the strip.
- Metadata rows use a fixed 12-col label pad. Skip any row whose value is empty rather than printing `None` — match `gh issue view` style.
- Horizontal rule = `─` × min(width, 80). Already what we do.
- Section headers ("Related MRs (3)", "Comments (5)") show counts inline. State glyphs in the related-MR block: `✓` merged / closed-good, `✗` closed-bad, `●` open, `◌` draft. Use existing logic in `FormatIssueDetail` (lines 277–282) — keep behavior, restyle the wrapper line.

## Implementation

### Step 1 — refactor `FormatIssueDetail`

In `internal/gitlab/issues.go`:

1. Add a `formatHeaderStrip(...)` helper that takes `(labels []labeled, width int)` where `labeled` is `struct{ k, v string }`, drops any row with `v == ""`, and writes `<padded-key><gap><value>\n` lines using the 12-col pad. The padding constant lives at the top of the file.
2. Inline-build the labeled list for issues: `Assignees`, `Labels`, `Milestone`, `Iteration` (with dates in parens), `Author`, `Updated`, `URL`. Drop the `Created` row — `Updated` is what users actually scan for staleness; `Created` is in the activity feed.
3. Replace the existing six-line block in `FormatIssueDetail` with `formatHeaderStrip(...)`.
4. Wrap the related-MRs block in a `Related MRs (<n>)` heading; keep the existing `stateIcon` logic.
5. Wrap the notes block in `Comments (<n>)` heading.

### Step 2 — mirror in `FormatMRDetail`

In `internal/gitlab/mergerequests.go`:

1. Use the same `formatHeaderStrip(...)` helper (move it to a new `internal/gitlab/format.go` so both files share it).
2. MR-specific labeled rows: `Assignee`, `Reviewers` (comma-joined), `Labels`, `Source`, `Target`, `Pipeline`, `Changes`, `Author`, `Updated`, `URL`. Skip empty values as above.
3. Reuse the Comments section formatter.

### Step 3 — title row enrichment

In the tab files (`internal/ui/tabs/issues.go:184`, `internal/ui/tabs/mergerequests.go` equivalent), change the title argument passed to `DetailPane.SetContent`:

- Issues: `fmt.Sprintf("#%d  %s %s  ·  %s", iid, stateGlyph(state), state, title)`
- MRs: `fmt.Sprintf("!%d  %s %s  ·  %s", iid, stateGlyph(state), state, title)`

`stateGlyph` is a small switch matching `theme.Color*` choices already in `messages.IssueState` etc. — put it in `internal/gitlab/format.go` alongside `formatHeaderStrip`.

### Step 4 — narrow-width fallback

If `width < 60`, drop the label pad to 0 and use `<key>: <value>\n` (current format). Saves us regressions on small terminals while keeping the structured look at typical widths.

## What stays the same

- `DetailPane` component — no changes.
- Cache schema — no changes. Every field shown in the new strip already exists on `messages.GitLabIssue` / `GitLabMR`.
- Tab `Update()` flow, `selectIssue` / `selectMR`, fetch debouncing — unchanged.
- Markdown rendering via glamour, relative-URL resolution, `Ctrl+click` URL opening — all preserved because the body text path is untouched.
- `SetContent`'s `(title, content)` contract — still string-based, no new component split.

## Test plan

- Manual: open an issue with full metadata (iteration, milestone, labels, multiple related MRs, many comments) — verify nothing is dropped vs. the old format.
- Manual: open an issue with mostly-empty metadata (no milestone, no iteration) — verify those rows are _omitted_, not printed as `None`.
- Manual: open in an 80-col terminal — verify narrow fallback engages cleanly.
- Manual: `Ctrl+click` a URL in the body still opens it.
- Manual: scroll with `j/k`, `gg`, `G` — same behavior.
- `task check` — gofmt, golangci-lint, build.

## What this plan deliberately does NOT do

Each is a separate follow-up doc, ordered by AI-handoff value:

1. **Checklist / "Jobs to be done" items** — highest-value addition for the Claude prompt. Needs a GitLab API call for task list items and a new cache table.
2. **Linked items with relation type + state badges** — `Blocked by` / `Blocks` / `Relates to` from the work-item links endpoint. New cache table; one block in the footer between Related MRs and Comments.
3. **Multi-assignee support** — `messages.GitLabIssue.Assignee string` → `Assignees []string`. Touches the cache schema (column → TEXT JSON), the `convertIssue` mapper, the AI-toggle logic in `toggleAIAssignee`, the export builders, and the query DSL's `assignee:` matcher. Latent bug for teams that pair on tickets — worth doing soon.
4. **Parent (epic) link** — single string, easy once the work-items API is wired.
5. **Weight / Due date** — surface inline in the sidebar (`due in 3d` chip) and in the header strip; add `due:<7d` and `weight:>3` DSL keys.
6. **Optional sticky right-strip at ≥160 cols** — only if anyone asks.

## Estimated diff

Roughly:

- `internal/gitlab/format.go` — new, ~50 lines (header strip helper + state glyph).
- `internal/gitlab/issues.go:FormatIssueDetail` — net -10 lines (delegation + heading rewrite).
- `internal/gitlab/mergerequests.go:FormatMRDetail` — net +10 lines (matches the new shape).
- `internal/ui/tabs/issues.go`, `internal/ui/tabs/mergerequests.go` — 1 line each (title format).

Single commit; no schema or message-type changes.
