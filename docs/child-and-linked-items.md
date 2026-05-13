# Child items, linked items, and work-item status/parent

## Goal

Surface the four data points GitLab shows in its right rail / mid-page that lazydev currently leaves as `—` placeholders or doesn't show at all:

1. **Linked items** with relation type — `Blocked by`, `Blocks`, `Relates to`, plus state badges (Done / Closed / Open / Draft). Today: missing.
2. **Child items** — work-item hierarchy children. Today: missing.
3. **Status** — work-item workflow status (`To do`, `In progress`, etc., distinct from open/closed). Today: header-strip placeholder.
4. **Parent** — parent work-item / epic title. Today: header-strip placeholder.

All four come from work-items / issue-links APIs that the standard `Issues.GetIssue` doesn't return.

Out of scope: editing (adding/removing links, changing parent, setting status). Read-only for v1. MRs do not get this — they don't have the work-item hierarchy.

## API surface — performance-first

**Goal**: every field rendered in the detail pane reads from the local cache. No per-detail-load API calls except as a freshness-refresh, same pattern that exists today for the basic issue fields.

To populate the cache efficiently we need a **bulk** path, not N+1. The SDK's per-item helpers exist but multiply request count:

| Approach                                                                                         | Calls per sync (N issues)                            | Verdict                                                    |
| ------------------------------------------------------------------------------------------------ | ---------------------------------------------------- | ---------------------------------------------------------- |
| REST `Issues.GetIssue` + `IssueLinks.ListIssueRelations` + SDK `WorkItems.GetWorkItem` per issue | **3 × N**                                            | Too slow.                                                  |
| Existing REST `Issues.ListProjectIssues` + GraphQL `WorkItem` per detail load only               | **N + on-demand**                                    | Pushes the cost to first-open of each item; UX still lags. |
| **GraphQL `project.workItems { … widgets … }` bulk paginated**                                   | **~N/50** (one page = 50 items, all widgets in-line) | What we want.                                              |

We pick the GraphQL bulk path. One request per page returns title / state / iid / status / parent / children / linked items for 50 work items at a time. For a typical 200-item prefetch that's **4 GraphQL requests** instead of 600 REST calls.

The query template (project-scoped, paginated via `endCursor`):

```graphql
query LazyDevWorkItems(
  $fullPath: ID!
  $first: Int!
  $after: String
  $updatedAfter: Time
) {
  project(fullPath: $fullPath) {
    workItems(
      first: $first
      after: $after
      sort: UPDATED_AT_ASC
      includeAncestors: true
      updatedAfter: $updatedAfter
    ) {
      pageInfo {
        hasNextPage
        endCursor
      }
      nodes {
        iid
        state
        title
        webUrl
        createdAt
        updatedAt
        workItemType {
          name
        }
        author {
          username
        }
        widgets {
          ... on WorkItemWidgetAssignees {
            assignees {
              nodes {
                username
              }
            }
          }
          ... on WorkItemWidgetLabels {
            labels {
              nodes {
                title
              }
            }
          }
          ... on WorkItemWidgetMilestone {
            milestone {
              title
            }
          }
          ... on WorkItemWidgetIteration {
            iteration {
              title
              startDate
              dueDate
            }
          }
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
          ... on WorkItemWidgetLinkedItems {
            linkedItems {
              nodes {
                linkType # "relates_to" | "blocks" | "is_blocked_by"
                workItem {
                  iid
                  title
                  state
                  webUrl
                }
              }
            }
          }
          ... on WorkItemWidgetDescription {
            description
          }
        }
      }
    }
  }
}
```

This single query replaces **all four** existing data sources for issues (`ListIssuesUpdatedAfter`, per-detail `GetIssue`, the new `ListIssueRelations`, and the new per-detail work-item GraphQL call). Notes/comments and related MRs are still fetched separately — they live on different widgets and grow unboundedly per item, so per-detail-load remains correct for those.

**Pagination**: `first: 50, after: <cursor>`; loop until `pageInfo.hasNextPage == false`. `updatedAfter` mirrors the existing incremental high-water mark.

**Premium-gated widgets**: GraphQL responds with the widget node missing from the array (not an error). All four optional widgets independently degrade to zero values, rendered as `—`.

**Authentication**: same PAT used today. GraphQL endpoint is at the host root (`/api/graphql`), the SDK exposes `c.Raw.GraphQL.Do(query, &result)`.

### Per-detail freshness

Detail load stays cache-first. To stay fresh on actively-viewed items we run a single-item version of the same GraphQL query when the user opens an issue. Same shape, `workItem(iid:)` instead of `workItems(...)` — one round trip refreshes everything except notes (which keep their separate per-detail REST fetch).

### Linked items: REST fallback

If a GitLab instance is on an older version where `WorkItemWidgetLinkedItems` is unavailable, fall back to `IssueLinks.ListIssueRelations` per detail load only. Detect via a one-shot `__schema { types { name } }` introspection check at client init; cache the result on the client. Default-on once confirmed.

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

**Prefetch** (first run / `task wipe-cache`):

1. Page through `project.workItems` 50 at a time, ordered by `updatedAt ASC`. Each page is one GraphQL round trip with all widgets in-line.
2. Inside one DB transaction per page, upsert `issues` (with the new `status` / `parent_iid` / `parent_title` columns and existing assignees/labels/iteration/milestone fields), `linked_items`, and `child_items`. Emit a `SyncEvent` with progress.
3. MRs use the existing REST path — unchanged.

**Incremental** (background tick, every ~20s):

1. Same paginated GraphQL query, but with `updatedAfter: <maxIssueUpdatedAt - 1m overlap>`. Typically returns ≤50 nodes → one request, fully populates the cache.
2. Notes are still fetched separately on detail load, as today.

**Detail load** (`selectIssue`):

1. Cache-first paint (everything is already in the cache from sync).
2. Background API refresh — runs the single-item GraphQL query plus the existing notes / related-MR REST calls. Results overwrite cached rows 1–2s later. Stale `fetchSeq` check stays.

**Why this is a strict win over per-detail enrichment**:

- First open of an item is **instant** (no widget fetch needed; it's already in the cache).
- Sidebar UI affordances that depend on widgets (e.g. "blocked" indicator) become viable because the data is bulk-available, not "fetch as you scroll."
- Request count per sync drops from O(N) to O(N/50) — a 200-item project goes from ~600 requests to ~4.
- One write path. No N+1 inside the detail load.

**Cost** vs. the deferred-fetch design:

- A bulk page is ~10–50 KB JSON for 50 work items with full widgets. ~5× the bytes of the current REST list. Negligible at any reasonable project size.
- The GraphQL response is more complex to map. Tests and a stable shim type help.

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
- Schema bump to `"3"`; add the three new columns to `issues`, and the two new tables (`linked_items`, `child_items`).
- `migrate()` already drops on version mismatch. Done.

### Step 2 — GraphQL client glue

- New file `internal/gitlab/workitems_graphql.go`.
- One exported function: `ListWorkItemsBulk(updatedAfter time.Time) iter.Seq2[[]workItem, error]` (or a callback-based variant) that pages through `project.workItems` using the template above, yielding each page until exhausted.
- One single-item function: `GetWorkItemBulk(iid int64) (workItem, error)` for the per-detail freshness path. Shares the response mapper.
- Internal `workItem` type matches the GraphQL response shape; an `intoMessages()` method splits it into `(GitLabIssue, []GitLabLinkedItem, []GitLabChildItem)` for the cache write.
- Use `c.Raw.GraphQL.Do(query, &result, options...)` (already in the SDK).

### Step 3 — cache plumbing

- Extend `UpsertIssues` to also accept linked + child items so we can write everything in one tx per page.
- Actually cleaner: new `UpsertWorkItemPage(ctx, []WorkItemPayload)` that internally splits into the three tables. Keep `UpsertIssues` for the MR/sync path that doesn't have widgets.
- New `ListLinkedItems(ctx, issueIID)` / `ListChildItems(ctx, parentIID)`.
- Extend `GetIssue` to return the linked + children slices alongside the existing data. Signature change.

### Step 4 — Syncer

- Replace the issues branch of `prefetch()` and `incremental()` with the new bulk GraphQL paginator. The MR branch is unchanged.
- `incremental()` uses `MaxIssueUpdatedAt - 1m overlap` as `updatedAfter`. First-run prefetch starts with `now - prefetchWindow`.
- Emit `SyncEvent{Kind: "issues", Progress: "<page>/<totalEstimated>"}` per page. Total estimate comes from the first page response if available, else we just stream count-so-far.

### Step 5 — Detail-load refresh

- `selectIssue`'s `apiCmd` switches from `c.Raw.Issues.GetIssue` to `c.GetWorkItemBulk(iid)` for the issue body + widgets, plus the existing notes / related-MR REST calls (parallel, batched).
- Cache write touches `issues`, `linked_items`, `child_items`, `notes`, `related_mrs` in one tx.
- `issueDetailResultMsg` carries the linked + children slices. Same `fetchSeq` staleness rules.

### Step 6 — Formatter

- Wire the header-strip `Status` and `Parent` rows.
- Two new footer sections: `Child items (N)` and `Linked items (N)`. The linked block sub-groups in fixed order: `Blocked by`, `Blocks`, `Relates to`.
- `formatLinkedItems(items) string` and `formatChildItems(items) string` helpers in `internal/gitlab/format.go`.

### Step 7 — Graceful degradation

GraphQL widget arrays can be missing entirely (Free-tier instances, older GitLab versions). Map each `... on WorkItemWidget*` block defensively:

- Missing widget → zero value for that field. No error.
- The whole query failing (network, auth, schema mismatch) → emit `SyncEvent{State: "offline", Err: …}` and back off as today.

For the older-GitLab linked-items fallback (Step 2 notes), introspect once at client init; if the linked-items widget isn't in the schema, the `ListWorkItemsBulk` mapper leaves the linked slice empty and we fall back to REST `IssueLinks.ListIssueRelations` per detail load. Old code path stays around precisely for this case.

### Step 8 — Query DSL (defer)

`blocked-by:#X` / `blocks:#X` / `parent:<title>` / `status:to-do` / `child-of:#X` are obvious DSL extensions now that the cache has the data. Out of scope for this plan; tracked in the follow-up section.

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
- `internal/cache/store.go` — new bulk-upsert helper, list helpers, GetIssue signature change, transactional write. ~150 lines.
- `internal/gitlab/workitems_graphql.go` — new file: bulk paginated GraphQL list + single-item fetch + response mapper. ~200 lines (biggest single file in the change).
- `internal/cache/sync.go` — replace REST issues path with the GraphQL bulk paginator; keep MR REST path. ~40 lines net change.
- `internal/gitlab/issues.go` — adjust `GetIssue` to delegate to `GetWorkItemBulk`, keep notes / related-MR REST calls. ~30 lines.
- `internal/gitlab/format.go` / `issues.go` — formatter for both new sections, wire Status/Parent rows. ~40 lines.
- `internal/ui/tabs/issues.go` — message type extension, plumb through. ~25 lines.
- `internal/cache/store_test.go` — new round-trip test for linked + children. ~40 lines.

Total: ~570 lines of net additions across 9 files; one schema bump. The diff is large enough to be worth splitting into three commits:

1. **Schema + types + cache helpers** — no behavior change yet; old REST sync still works.
2. **GraphQL client + Syncer switch** — replace the REST issues path. Tested standalone with `task wipe-cache && task run`.
3. **UI: header rows + footer sections** — render the now-cached data.

This ordering means each commit can be reverted independently if needed, and `git bisect` can pinpoint where any regression entered.
