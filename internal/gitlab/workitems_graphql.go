package gitlab

import (
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
			"iid":      strconv.FormatInt(iid, 10),
		},
	}
	var resp workItemsByIIDResponse
	if _, err := c.Raw.GraphQL.Do(q, &resp); err != nil {
		return nil, err
	}
	if err := resp.firstError(); err != nil {
		return nil, err
	}
	projectGID := resp.Data.Project.ID
	nodes := resp.Data.Project.Issues.Nodes
	if len(nodes) == 0 {
		return nil, nil
	}
	bundle := nodes[0].toBundle(projectGID)
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
	proj := resp.Data.Project
	out := make([]wiBundle, 0, len(proj.Issues.Nodes))
	for _, n := range proj.Issues.Nodes {
		out = append(out, n.toBundle(proj.ID))
	}
	next := ""
	if proj.Issues.PageInfo.HasNextPage {
		next = proj.Issues.PageInfo.EndCursor
	}
	return out, next, nil
}

// --- GraphQL response types ---

type workItemsPageResponse struct {
	Data struct {
		Project struct {
			ID     string `json:"id"`
			Issues struct {
				PageInfo struct {
					HasNextPage bool   `json:"hasNextPage"`
					EndCursor   string `json:"endCursor"`
				} `json:"pageInfo"`
				Nodes []wiNode `json:"nodes"`
			} `json:"issues"`
		} `json:"project"`
	} `json:"data"`
	Errors gqlErrors `json:"errors"`
}

func (r workItemsPageResponse) firstError() error { return r.Errors.first() }

type workItemsByIIDResponse struct {
	Data struct {
		Project struct {
			ID     string `json:"id"`
			Issues struct {
				Nodes []wiNode `json:"nodes"`
			} `json:"issues"`
		} `json:"project"`
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

// wiNode is the per-issue GraphQL payload. The project here is a
// classic Issue (not a WorkItem) because lazydev's UI is rooted in
// the issue tracker — work-item-only types (Tasks under a parent,
// Objectives, Key Results) are surfaced as child rows rather than as
// top-level sidebar entries.
type wiNode struct {
	IID         string        `json:"iid"`
	State       string        `json:"state"`
	Title       string        `json:"title"`
	WebURL      string        `json:"webUrl"`
	CreatedAt   time.Time     `json:"createdAt"`
	UpdatedAt   time.Time     `json:"updatedAt"`
	Description string        `json:"description"`
	Author      *userNode     `json:"author"`
	Labels      labelConn     `json:"labels"`
	Milestone   *titledNode   `json:"milestone"`
	Iteration   *iterationGQL `json:"iteration"`
	Assignees   userConn      `json:"assignees"`

	// Work-item-only data accessible via the Issue.workItemWidgets path
	// in GraphQL — gated behind feature flags on some GitLab versions,
	// which is why we read it via separate fields rather than a typed
	// widgets[] discriminator. Each pointer is nil when absent.
	WorkItem *wiWidgetsNode `json:"workItem"`
}

// wiWidgetsNode collects the fields we read from the WorkItem path on
// an Issue. Fields are zero-value when not granted (Premium gating,
// older GitLab versions).
type wiWidgetsNode struct {
	Status   *titledNode    `json:"workItemStatus"`
	Parent   *parentNode    `json:"parent"`
	Children childConn      `json:"children"`
	LinkedTo linkedItemConn `json:"linkedItems"`
}

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
// projectGID is the parent project's GID; we parse the numeric tail so
// formatters can resolve /uploads/ paths.
func (n wiNode) toBundle(projectGID string) wiBundle {
	b := wiBundle{
		Issue: messages.GitLabIssue{
			IID:         atoi64(n.IID),
			ProjectID:   gidToInt64(projectGID),
			Title:       n.Title,
			State:       gqlStateToREST(n.State),
			Description: n.Description,
			WebURL:      n.WebURL,
			CreatedAt:   n.CreatedAt,
			UpdatedAt:   n.UpdatedAt,
		},
	}
	if n.Author != nil {
		b.Issue.Author = n.Author.Username
	}
	if len(n.Assignees.Nodes) > 0 {
		b.Issue.Assignees = make([]string, 0, len(n.Assignees.Nodes))
		for _, a := range n.Assignees.Nodes {
			b.Issue.Assignees = append(b.Issue.Assignees, a.Username)
		}
	}
	if len(n.Labels.Nodes) > 0 {
		b.Issue.Labels = make([]string, 0, len(n.Labels.Nodes))
		for _, l := range n.Labels.Nodes {
			b.Issue.Labels = append(b.Issue.Labels, l.display())
		}
	}
	if n.Milestone != nil {
		b.Issue.Milestone = n.Milestone.display()
	}
	if n.Iteration != nil {
		b.Issue.Iteration = n.Iteration.Title
		b.Issue.IterationDates = formatIterationDates(n.Iteration.StartDate, n.Iteration.DueDate)
	}
	if n.WorkItem != nil {
		b.Issue.Status = n.WorkItem.Status.display()
		if n.WorkItem.Parent != nil {
			b.Issue.ParentIID = atoi64(n.WorkItem.Parent.IID)
			b.Issue.ParentTitle = n.WorkItem.Parent.Title
		}
		for _, c := range n.WorkItem.Children.Nodes {
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
		for _, li := range n.WorkItem.LinkedTo.Nodes {
			b.Linked = append(b.Linked, messages.GitLabLinkedItem{
				IID:      atoi64(li.WorkItem.IID),
				Title:    li.WorkItem.Title,
				State:    gqlStateToREST(li.WorkItem.State),
				LinkType: gqlLinkType(li.LinkType),
				WebURL:   li.WorkItem.WebURL,
			})
		}
	}
	return b
}

// --- helpers ---

// gqlStateToREST maps the GraphQL state spelling ("OPENED", "CLOSED")
// to the REST spelling ("opened", "closed") that the rest of lazydev
// already uses everywhere.
func gqlStateToREST(s string) string {
	return strings.ToLower(s)
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

// gidToInt64 parses the trailing integer from a GitLab GID
// (e.g. "gid://gitlab/Project/12345678" → 12345678).
func gidToInt64(gid string) int64 {
	idx := strings.LastIndex(gid, "/")
	if idx < 0 || idx == len(gid)-1 {
		return 0
	}
	return atoi64(gid[idx+1:])
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

// workItemsPageQuery fetches one page of issues with every widget
// field lazydev renders. The `workItem` block is the work-items
// projection of the same Issue — it surfaces Status, Parent, Children,
// and LinkedItems in one round trip.
const workItemsPageQuery = `
query LazyDevWorkItemsPage($fullPath: ID!, $first: Int!, $after: String, $updatedAfter: Time) {
  project(fullPath: $fullPath) {
    id
    issues(
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
        description
        author { username }
        assignees { nodes { username } }
        labels { nodes { title } }
        milestone { title }
        iteration { title startDate dueDate }
        workItem {
          workItemStatus { name }
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
              webUrl
              workItemType { name }
            }
          }
          linkedItems {
            nodes {
              linkType
              workItem {
                iid
                title
                state
                webUrl
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
// freshness. Same projection shape as workItemsPageQuery, narrowed by
// IID so it returns a single-element nodes array.
const workItemsByIIDQuery = `
query LazyDevWorkItemByIID($fullPath: ID!, $iid: String!) {
  project(fullPath: $fullPath) {
    id
    issues(iids: [$iid]) {
      nodes {
        iid
        state
        title
        webUrl
        createdAt
        updatedAt
        description
        author { username }
        assignees { nodes { username } }
        labels { nodes { title } }
        milestone { title }
        iteration { title startDate dueDate }
        workItem {
          workItemStatus { name }
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
              webUrl
              workItemType { name }
            }
          }
          linkedItems {
            nodes {
              linkType
              workItem {
                iid
                title
                state
                webUrl
              }
            }
          }
        }
      }
    }
  }
}
`
