# Child items, linked items, and work-item status/parent

## Goal

Surface the four data points GitLab shows in its right rail / mid-page that lazydev currently leaves as `—` placeholders or doesn't show at all:

1. **Linked items** with relation type — `Blocked by`, `Blocks`, `Relates to`, plus state badges (Done / Closed / Open / Draft). Today: missing.
2. **Child items** — work-item hierarchy children. Today: missing.
3. **Status** — work-item workflow status (`To do`, `In progress`, etc., distinct from open/closed). Today: header-strip placeholder.
4. **Parent** — parent work-item / epic title. Today: header-strip placeholder.

All four come from work-items / issue-links APIs that the standard `Issues.GetIssue` doesn't return.

Out of scope: editing (adding/removing links, changing parent, setting status). Read-only for v1. MRs do not get this — they don't have the work-item hierarchy.

## API surface

The Go SDK (`gitlab.com/gitlab-org/api/client-go@v1.46.0`) covers most of what we need:

- **Linked items (REST)**: `IssueLinks.ListIssueRelations(pid, iid)` → `[]*IssueRelation` with `LinkType` (`"relates_to"` | `"blocks"` | `"is_blocked_by"`), `IID`, `Title`, `State`, `WebURL`, `Labels`. One-shot call, no pagination concerns at typical sizes.
- **Work item (GraphQL)**: `WorkItems.GetWorkItem(fullPath, iid)` → `*WorkItem` with `Status` and basic fields. The SDK's built-in GraphQL **template does not currently include the hierarchy widget**, so children / parent come back nil. We have two options:
  - (A) Use `WorkItems.GetWorkItem` for Status only; do a second raw GraphQL call (via `c.Raw.GraphQL.Do`) with our own template that fetches `hierarchyWidget { parent { … }, children { nodes { … } } }`.
  - (B) Skip the SDK helper and run a single GraphQL query that fetches Status + Parent + Children in one round trip.

Pick **(B)** — one call instead of two, and the SDK's `Status` template is shallow enough that doing it ourselves isn't more work. We get the GraphQL primitive (`c.Raw.GraphQL.Do(query, &result)`) for free.

GraphQL query shape:

```graphql
query LazyDevWorkItem($fullPath: ID!, $iid: String!) {
  namespace(fullPath: $fullPath) {
    workItem(iid: $iid) {
      iid
      state
      widgets {
        ... on WorkItemWidgetStatus {
          status {
            name
          }
        }
        ... on WorkItemWidgetHierarchy {
          parent {
            iid
            title
            webUrl
          }
          children {
            nodes {
              iid
              title
              state
              workItemType {
                name
              }
              webUrl
            }
          }
        }
      }
    }
  }
}
```

**Caveat**: GitLab Premium gates some widgets (hierarchy beyond direct parent/child, status names). The query should degrade gracefully — fields just return null, which we render as `—`.

## Caching

Two new tables, both write-on-detail-fetch (like `related_mrs` today):

```sql
CREATE TABLE IF NOT EXISTS linked_items (
    issue_iid    INTEGER NOT NULL,
    target_iid   INTEGER NOT NULL,
    link_type    TEXT    NOT NULL DEFAULT '',  -- "blocks" | "is_blocked_by" | "relates_to"
    title        TEXT    NOT NULL DEFAULT '',
    state        TEXT    NOT NULL DEFAULT '',
    web_url      TEXT    NOT NULL DEFAULT '',
    PRIMARY KEY (issue_iid, target_iid)
);

CREATE TABLE IF NOT EXISTS child_items (
    parent_iid   INTEGER NOT NULL,
    child_iid    INTEGER NOT NULL,
    title        TEXT    NOT NULL DEFAULT '',
    state        TEXT    NOT NULL DEFAULT '',
    web_url      TEXT    NOT NULL DEFAULT '',
    item_type    TEXT    NOT NULL DEFAULT '',  -- "Issue" | "Task" | "Objective" | …
    PRIMARY KEY (parent_iid, child_iid)
);
```

Status and parent are scalar — they go on `messages.GitLabIssue` and the `issues` table as new columns:

```sql
ALTER TABLE issues ADD COLUMN status      TEXT NOT NULL DEFAULT '';
ALTER TABLE issues ADD COLUMN parent_iid  INTEGER NOT NULL DEFAULT 0;
ALTER TABLE issues ADD COLUMN parent_title TEXT NOT NULL DEFAULT '';
```

Bump `currentSchemaVersion` from `"2"` → `"3"`. The existing `migrate()` already drops the data tables on a version mismatch — no extra logic needed; the Syncer repopulates from GitLab.

## Sync strategy

**On every detail load (the existing `selectIssue` flow):**

1. Call `c.Raw.IssueLinks.ListIssueRelations(projectID, iid)` → upsert `linked_items` rows.
2. Run the GraphQL `LazyDevWorkItem` query → upsert `child_items` rows + write `status` / `parent_iid` / `parent_title` onto the issue row.
3. The detail-pane render reads from the cache (same `selectIssue` → cache-first-then-API pattern that exists today). First paint comes from whatever's already cached; the API result overwrites 1–2s later.

**On prefetch / incremental sync:** do **not** fetch links/children/status. They're per-item N+1 calls — too expensive for the bulk pass. The header strip's `Status` / `Parent` rows stay as `—` until the user opens the item. That's an acceptable trade for not multiplying request counts by ~5×.

## UI

### Header strip

`Status` and `Parent` rows already exist as placeholders. Wire them:

```go
{"Status",  issue.Status},
{"Parent",  formatParent(issue.ParentIID, issue.ParentTitle)},
```

`formatParent` returns `"#2100 Inference improvements"` when set, empty otherwise (rendered as `—` by the strip helper). Parent label remains unlinked text for v1 — clickable navigation is a follow-up.

### Footer sections

New blocks slotted between "Related MRs" and "Comments" in the detail pane:

```
──────────────────────────────────────────
Child items (3)
  ● #2410 Build snapshot store           opened
  ✓ #2403 Wire snapshot to manager       closed
  ✓ #2398 RFC: snapshot lifecycle        closed
──────────────────────────────────────────
Linked items (3)
  Blocked by
    ✓ #2490 Integrate AsyncChunk…        closed
    ✓ #2489 alpha_gym: Rename RTC…       closed
    ✓ #2498 Phase 1: Action chunk…       closed
  Blocks
    ● #2510 Phase 3: chunk metrics       opened
  Relates to
    ● #2480 Async stepper rollout        opened
```

Section order: state glyph (`✓`/`✗`/`●`/`◌`) + IID + title + trailing state word. `commentSep`-style thin rule between sub-groups of linked items would be overkill — single-line group headers (`Blocked by` / `Blocks` / `Relates to`) are enough. Render groups in fixed order: `Blocked by` first (most actionable), then `Blocks`, then `Relates to`. Empty sub-groups omitted.

Both sections are skipped entirely when the cached arrays are empty.

### Sidebar (deferred)

Showing a "🚫 blocked" indicator on sidebar rows when an item has open `is_blocked_by` links is tempting but out of scope. The cache supports it; UI work goes in a follow-up.

## Implementation steps

### Step 1 — types and schema

- Add `Status string`, `ParentIID int64`, `ParentTitle string` to `messages.GitLabIssue`.
- Add `GitLabLinkedItem` and `GitLabChildItem` types alongside `GitLabIssueMR` in `pkg/messages/messages.go`.
- Schema bump to `"3"`; add the three new columns to `issues`, and the two new tables.

### Step 2 — cache plumbing

- Extend `UpsertIssues` to write the three new columns; extend `scanIssue` to read them.
- New methods: `UpsertLinkedItems(ctx, issueIID, items)`, `UpsertChildItems(ctx, parentIID, items)`, plus list/get helpers.
- Extend `GetIssue` to return `(*GitLabIssue, []GitLabNote, []GitLabIssueMR, []GitLabLinkedItem, []GitLabChildItem, error)`. Yes, the signature grows. The alternative is a struct return — defer that refactor.

### Step 3 — GitLab client

- New file `internal/gitlab/workitems.go` with `GetWorkItemDetails(iid)` running the GraphQL query above and returning `(status string, parent *GitLabIssueMR-like, children []GitLabChildItem, err error)`. Project's `fullPath` is already on the client (resolved from the git remote).
- Extend the existing `(*Client).GetIssue` to also call `IssueLinks.ListIssueRelations` and the new `GetWorkItemDetails`. Return the combined data.

### Step 4 — formatter

- Update `FormatIssueDetail` to render the two new footer sections.
- New helper `formatLinkedItems` that groups by `LinkType` and prints fixed-order sub-groups.
- The header-strip `Status` and `Parent` rows already exist; the values just need to be passed in.

### Step 5 — tabs

- `selectIssue` already does `cacheCmd + apiCmd`; the API command's return type grows to include linked + children. Cache write becomes one transaction that touches `issues`, `notes`, `related_mrs`, `linked_items`, `child_items`. Wrap in a single `tx` for atomicity.
- `issueDetailResultMsg` gains `linked []messages.GitLabLinkedItem` and `children []messages.GitLabChildItem` fields. The detail handler hands all of it to `FormatIssueDetail`.

### Step 6 — graceful degradation

GraphQL fields can return null (Premium-gated widgets, deleted parents, etc.). All four data points are independently optional:

- Linked items REST call failing → log warning, keep going.
- GraphQL call failing → log warning, keep `status` / `parent` / `children` as their zero values.
- Don't fail the whole detail load on either.

### Step 7 — query DSL (defer)

`blocked-by:#X` / `blocks:#X` / `parent:<title>` / `status:to-do` are obvious DSL extensions. None of them are in scope for this plan — they need their own design (especially the `#IID` literal syntax in tokens, which we don't have today). Mention in the follow-up section.

## Test plan

- Manual: open an issue with multiple `Blocked by` and one `Blocks` link — verify groupings, ordering, state glyphs.
- Manual: open an issue with no links and no children — verify both sections are entirely omitted (no empty headers).
- Manual: open an issue with a parent — verify `Parent` row shows `#<iid> <title>`.
- Manual: open an issue with status `In progress` — verify `Status` row shows that exact name.
- Manual: open an issue on a Free-tier GitLab project (hierarchy widget likely null) — verify `Parent` and child items render as `—` / omitted, no crash.
- Manual: `task wipe-cache && lazydev` — verify prefetch completes; opening an item then triggers the new fetches and rows appear.
- Unit: cache round-trip for the two new tables (extend `store_test.go`).
- `task check`.

## What this plan deliberately defers

1. **Sidebar "blocked" indicator** — show 🚫 on sidebar rows that have open `is_blocked_by` links.
2. **Query DSL extensions** — `blocked-by:`, `blocks:`, `parent:`, `status:`, `child-of:`.
3. **Editing** — adding/removing links, changing parent, setting status. The SDK has `CreateIssueLink` / `DeleteIssueLink`; status writes go through GraphQL mutations.
4. **Clickable parent / children** — Ctrl+click to open the linked detail directly within lazydev (instead of just the browser).
5. **MR linked items** — MRs can have related issues, but the relationship model is different (issue mentions vs typed links). Out of scope.

## Estimated diff

- `pkg/messages/messages.go` — +3 fields on issue, two new types. ~25 lines.
- `internal/cache/schema.go` — +3 columns, +2 tables, bump version. ~20 lines.
- `internal/cache/store.go` — new upsert / scan helpers, GetIssue signature change, transactional write. ~120 lines.
- `internal/gitlab/workitems.go` — new file, raw GraphQL call. ~80 lines.
- `internal/gitlab/issues.go` — extend `GetIssue` to call the two new APIs. ~30 lines.
- `internal/gitlab/format.go` / `issues.go` — formatter for both sections, wire Status/Parent rows. ~40 lines.
- `internal/ui/tabs/issues.go` — message type extension, plumb through. ~25 lines.
- `internal/cache/store_test.go` — new round-trip test. ~30 lines.

Total: ~370 lines of net additions across 8 files; one schema bump. Single commit, or split into "schema + types" → "client + cache" → "UI" if the diff gets unwieldy mid-implementation.
