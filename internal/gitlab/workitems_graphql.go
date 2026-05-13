package gitlab

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	"github.com/abhishek-rana/lazydev/pkg/messages"
)

// wiBundle is the mapped result of a single work-item GraphQL node:
// the flat issue row plus the two related sets (linked / children).
type wiBundle struct {
	Issue    messages.GitLabIssue
	Linked   []messages.GitLabLinkedItem
	Children []messages.GitLabChildItem
}

const workItemsPageSize = 50

// StreamWorkItemsUpdatedAfter paginates through the project's work
// items via one GraphQL query per page (each carrying every widget we
// render). The onPage callback runs once per page so the caller can
// upsert and emit progress events without buffering everything in
// memory.
//
// updatedAfter zero fetches every work item; otherwise narrows to
// updatedAt > t (used for incremental sync). Implements the
// cache.Source interface extension consumed by the Syncer.
func (c *Client) StreamWorkItemsUpdatedAfter(updatedAfter time.Time, onPage func(page messages.WorkItemPage) error) error {
	cursor := ""
	for {
		bundles, next, err := c.workItemsPage(cursor, updatedAfter)
		if err != nil {
			return err
		}
		if len(bundles) == 0 && next == "" {
			return nil
		}
		page := bundlesToPage(bundles)
		if err := onPage(page); err != nil {
			return err
		}
		if next == "" {
			return nil
		}
		cursor = next
	}
}

// bundlesToPage flattens the per-node bundles into the parallel slice
// + per-IID maps that the Syncer consumes.
func bundlesToPage(bs []wiBundle) messages.WorkItemPage {
	page := messages.WorkItemPage{
		Issues:   make([]messages.GitLabIssue, 0, len(bs)),
		Linked:   make(map[int64][]messages.GitLabLinkedItem),
		Children: make(map[int64][]messages.GitLabChildItem),
	}
	for _, b := range bs {
		page.Issues = append(page.Issues, b.Issue)
		if len(b.Linked) > 0 {
			page.Linked[b.Issue.IID] = b.Linked
		}
		if len(b.Children) > 0 {
			page.Children[b.Issue.IID] = b.Children
		}
	}
	return page
}

// GetWorkItemBundle fetches a single work item with all widgets. Used
// by the per-detail freshness path. Returns nil if the IID doesn't
// resolve (e.g. permission denied).
func (c *Client) GetWorkItemBundle(iid int64) (*wiBundle, error) {
	q := gitlab.GraphQLQuery{
		Query: workItemsByIIDQuery,
		Variables: map[string]any{
			"fullPath": c.ProjectID,
			"iids":     []string{strconv.FormatInt(iid, 10)},
		},
	}
	var resp workItemsByIIDResponse
	if _, err := c.Raw.GraphQL.Do(q, &resp); err != nil {
		return nil, err
	}
	if err := resp.firstError(); err != nil {
		return nil, err
	}
	nodes := resp.Data.Namespace.WorkItems.Nodes
	if len(nodes) == 0 {
		return nil, nil
	}
	bundle := nodes[0].toBundle(c.ProjectNumericID)
	return &bundle, nil
}

// workItemsPage runs one page of the bulk query and returns the
// mapped bundles plus the next cursor (empty when exhausted).
func (c *Client) workItemsPage(after string, updatedAfter time.Time) ([]wiBundle, string, error) {
	vars := map[string]any{
		"fullPath": c.ProjectID,
		"first":    workItemsPageSize,
	}
	if after != "" {
		vars["after"] = after
	}
	if !updatedAfter.IsZero() {
		vars["updatedAfter"] = updatedAfter.UTC().Format(time.RFC3339)
	}
	q := gitlab.GraphQLQuery{Query: workItemsPageQuery, Variables: vars}

	var resp workItemsPageResponse
	if _, err := c.Raw.GraphQL.Do(q, &resp); err != nil {
		return nil, "", err
	}
	if err := resp.firstError(); err != nil {
		return nil, "", err
	}
	nodes := resp.Data.Namespace.WorkItems.Nodes
	out := make([]wiBundle, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, n.toBundle(c.ProjectNumericID))
	}
	next := ""
	if resp.Data.Namespace.WorkItems.PageInfo.HasNextPage {
		next = resp.Data.Namespace.WorkItems.PageInfo.EndCursor
	}
	return out, next, nil
}

// --- GraphQL response types ---

type workItemsPageResponse struct {
	Data struct {
		Namespace struct {
			WorkItems struct {
				PageInfo struct {
					HasNextPage bool   `json:"hasNextPage"`
					EndCursor   string `json:"endCursor"`
				} `json:"pageInfo"`
				Nodes []wiNode `json:"nodes"`
			} `json:"workItems"`
		} `json:"namespace"`
	} `json:"data"`
	Errors gqlErrors `json:"errors"`
}

func (r workItemsPageResponse) firstError() error { return r.Errors.first() }

type workItemsByIIDResponse struct {
	Data struct {
		Namespace struct {
			WorkItems struct {
				Nodes []wiNode `json:"nodes"`
			} `json:"workItems"`
		} `json:"namespace"`
	} `json:"data"`
	Errors gqlErrors `json:"errors"`
}

func (r workItemsByIIDResponse) firstError() error { return r.Errors.first() }

type gqlErrors []struct {
	Message string `json:"message"`
}

func (e gqlErrors) first() error {
	if len(e) == 0 {
		return nil
	}
	msgs := make([]string, 0, len(e))
	for _, x := range e {
		msgs = append(msgs, x.Message)
	}
	return errors.New("graphql: " + strings.Join(msgs, "; "))
}

// wiNode is the GraphQL payload for a single work item. Widgets is a
// polymorphic array — every entry has a __typename plus the fields
// for its variant; we unmarshal with one flat struct since GraphQL
// returns them merged.
type wiNode struct {
	IID       string     `json:"iid"`
	State     string     `json:"state"`
	Title     string     `json:"title"`
	WebURL    string     `json:"webUrl"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	Author    *userNode  `json:"author"`
	Widgets   []wiWidget `json:"widgets"`
}

// wiWidget is a flat union of every widget field we read. Each widget
// object only populates the fields for its own __typename; the rest
// are nil. Allocating one struct shape avoids a custom UnmarshalJSON.
type wiWidget struct {
	Typename string `json:"__typename"`

	// WorkItemWidgetDescription
	Description string `json:"description"`

	// WorkItemWidgetAssignees
	Assignees userConn `json:"assignees"`

	// WorkItemWidgetLabels
	Labels labelConn `json:"labels"`

	// WorkItemWidgetMilestone
	Milestone *titledNode `json:"milestone"`

	// WorkItemWidgetIteration
	Iteration *iterationGQL `json:"iteration"`

	// WorkItemWidgetStatus
	Status *titledNode `json:"status"`

	// WorkItemWidgetHierarchy
	Parent   *parentNode `json:"parent"`
	Children childConn   `json:"children"`

	// WorkItemWidgetLinkedItems
	LinkedItems linkedItemConn `json:"linkedItems"`
}

// MarshalJSON / UnmarshalJSON are not customised — the default
// behaviour (unmarshal whichever fields are present) gives us the
// effect we want without per-widget structs.

// Used by the JSON decoder via Go's struct tag machinery; no methods
// required. The blank imports here document the encoding/json
// dependency for the file.
var _ = json.Unmarshal

type userNode struct {
	Username string `json:"username"`
}

type userConn struct {
	Nodes []userNode `json:"nodes"`
}

type titledNode struct {
	Title string `json:"title"`
	Name  string `json:"name"`
}

func (t *titledNode) display() string {
	if t == nil {
		return ""
	}
	if t.Name != "" {
		return t.Name
	}
	return t.Title
}

type labelConn struct {
	Nodes []titledNode `json:"nodes"`
}

type iterationGQL struct {
	Title     string `json:"title"`
	StartDate string `json:"startDate"`
	DueDate   string `json:"dueDate"`
}

type parentNode struct {
	IID    string `json:"iid"`
	Title  string `json:"title"`
	WebURL string `json:"webUrl"`
}

type childConn struct {
	Nodes []childNode `json:"nodes"`
}

type childNode struct {
	IID          string      `json:"iid"`
	Title        string      `json:"title"`
	State        string      `json:"state"`
	WebURL       string      `json:"webUrl"`
	WorkItemType *titledNode `json:"workItemType"`
}

type linkedItemConn struct {
	Nodes []linkedItemNode `json:"nodes"`
}

type linkedItemNode struct {
	LinkType string `json:"linkType"`
	WorkItem struct {
		IID    string `json:"iid"`
		Title  string `json:"title"`
		State  string `json:"state"`
		WebURL string `json:"webUrl"`
	} `json:"workItem"`
}

// toBundle splits the GraphQL node into the three cache rows.
// projectNumericID is the project's numeric REST ID — formatters need
// it to resolve /uploads/ paths in markdown bodies.
func (n wiNode) toBundle(projectNumericID int64) wiBundle {
	b := wiBundle{
		Issue: messages.GitLabIssue{
			IID:       atoi64(n.IID),
			ProjectID: projectNumericID,
			Title:     n.Title,
			State:     gqlStateToREST(n.State),
			WebURL:    n.WebURL,
			CreatedAt: n.CreatedAt,
			UpdatedAt: n.UpdatedAt,
		},
	}
	if n.Author != nil {
		b.Issue.Author = n.Author.Username
	}

	for _, w := range n.Widgets {
		switch w.Typename {
		case "WorkItemWidgetDescription":
			b.Issue.Description = w.Description

		case "WorkItemWidgetAssignees":
			if len(w.Assignees.Nodes) > 0 {
				b.Issue.Assignees = make([]string, 0, len(w.Assignees.Nodes))
				for _, a := range w.Assignees.Nodes {
					b.Issue.Assignees = append(b.Issue.Assignees, a.Username)
				}
			}

		case "WorkItemWidgetLabels":
			if len(w.Labels.Nodes) > 0 {
				b.Issue.Labels = make([]string, 0, len(w.Labels.Nodes))
				for _, l := range w.Labels.Nodes {
					b.Issue.Labels = append(b.Issue.Labels, l.display())
				}
			}

		case "WorkItemWidgetMilestone":
			b.Issue.Milestone = w.Milestone.display()

		case "WorkItemWidgetIteration":
			if w.Iteration != nil {
				b.Issue.Iteration = w.Iteration.Title
				b.Issue.IterationDates = formatIterationDates(w.Iteration.StartDate, w.Iteration.DueDate)
			}

		case "WorkItemWidgetStatus":
			b.Issue.Status = w.Status.display()

		case "WorkItemWidgetHierarchy":
			if w.Parent != nil {
				b.Issue.ParentIID = atoi64(w.Parent.IID)
				b.Issue.ParentTitle = w.Parent.Title
			}
			for _, c := range w.Children.Nodes {
				itemType := ""
				if c.WorkItemType != nil {
					itemType = c.WorkItemType.display()
				}
				b.Children = append(b.Children, messages.GitLabChildItem{
					IID:      atoi64(c.IID),
					Title:    c.Title,
					State:    gqlStateToREST(c.State),
					ItemType: itemType,
					WebURL:   c.WebURL,
				})
			}

		case "WorkItemWidgetLinkedItems":
			for _, li := range w.LinkedItems.Nodes {
				b.Linked = append(b.Linked, messages.GitLabLinkedItem{
					IID:      atoi64(li.WorkItem.IID),
					Title:    li.WorkItem.Title,
					State:    gqlStateToREST(li.WorkItem.State),
					LinkType: gqlLinkType(li.LinkType),
					WebURL:   li.WorkItem.WebURL,
				})
			}
		}
	}
	return b
}

// --- helpers ---

// gqlStateToREST maps the GraphQL state spelling ("OPEN", "CLOSED")
// to the REST spelling ("opened", "closed") that the rest of lazydev
// already uses everywhere. GraphQL uses "OPEN" for Issues but "open"
// for some other contexts — normalise to lowercase and then translate
// "open" → "opened" to match the REST convention.
func gqlStateToREST(s string) string {
	low := strings.ToLower(s)
	if low == "open" {
		return "opened"
	}
	return low
}

// gqlLinkType normalises the linkType enum to the lowercase REST
// spelling we already store ("blocks", "is_blocked_by", "relates_to").
func gqlLinkType(s string) string {
	return strings.ToLower(s)
}

// atoi64 parses a numeric string; returns 0 on failure (used for
// best-effort conversion of GraphQL IID strings).
func atoi64(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

// formatIterationDates renders "Mar 22 – Apr 4, 2026" from two ISO
// date strings. Empty when either is missing.
func formatIterationDates(start, due string) string {
	s, err1 := time.Parse("2006-01-02", start)
	d, err2 := time.Parse("2006-01-02", due)
	if err1 != nil || err2 != nil {
		return ""
	}
	return fmt.Sprintf("%s – %s", s.Format("Jan 2"), d.Format("Jan 2, 2006"))
}

// --- query templates ---

// workItemsPageQuery fetches one page of work items (filtered to
// type=ISSUE) with every widget lazydev renders. The widgets[] array
// is polymorphic — each entry has __typename plus the fields for its
// variant; the Go side dispatches on Typename when mapping.
const workItemsPageQuery = `
query LazyDevWorkItemsPage($fullPath: ID!, $first: Int!, $after: String, $updatedAfter: Time) {
  namespace(fullPath: $fullPath) {
    workItems(
      types: [ISSUE]
      first: $first
      after: $after
      sort: UPDATED_DESC
      updatedAfter: $updatedAfter
    ) {
      pageInfo { hasNextPage endCursor }
      nodes {
        iid
        state
        title
        webUrl
        createdAt
        updatedAt
        author { username }
        widgets {
          __typename
          ... on WorkItemWidgetDescription { description }
          ... on WorkItemWidgetAssignees { assignees { nodes { username } } }
          ... on WorkItemWidgetLabels    { labels    { nodes { title } } }
          ... on WorkItemWidgetMilestone { milestone { title } }
          ... on WorkItemWidgetIteration { iteration { title startDate dueDate } }
          ... on WorkItemWidgetStatus    { status    { name } }
          ... on WorkItemWidgetHierarchy {
            parent   { iid title webUrl }
            children { nodes { iid title state webUrl workItemType { name } } }
          }
          ... on WorkItemWidgetLinkedItems {
            linkedItems {
              nodes {
                linkType
                workItem { iid title state webUrl }
              }
            }
          }
        }
      }
    }
  }
}
`

// workItemsByIIDQuery is the single-item variant used for per-detail
// freshness. Same projection shape, filtered by IID.
const workItemsByIIDQuery = `
query LazyDevWorkItemByIID($fullPath: ID!, $iids: [String!]!) {
  namespace(fullPath: $fullPath) {
    workItems(types: [ISSUE], iids: $iids) {
      nodes {
        iid
        state
        title
        webUrl
        createdAt
        updatedAt
        author { username }
        widgets {
          __typename
          ... on WorkItemWidgetDescription { description }
          ... on WorkItemWidgetAssignees { assignees { nodes { username } } }
          ... on WorkItemWidgetLabels    { labels    { nodes { title } } }
          ... on WorkItemWidgetMilestone { milestone { title } }
          ... on WorkItemWidgetIteration { iteration { title startDate dueDate } }
          ... on WorkItemWidgetStatus    { status    { name } }
          ... on WorkItemWidgetHierarchy {
            parent   { iid title webUrl }
            children { nodes { iid title state webUrl workItemType { name } } }
          }
          ... on WorkItemWidgetLinkedItems {
            linkedItems {
              nodes {
                linkType
                workItem { iid title state webUrl }
              }
            }
          }
        }
      }
    }
  }
}
`
