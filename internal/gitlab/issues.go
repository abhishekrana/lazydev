package gitlab

import (
	"fmt"
	"strings"
	"time"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	"github.com/abhishek-rana/lazydev/pkg/messages"
)

// ListMyIssues returns issues assigned to, created by, or mentioning the authenticated user.
func (c *Client) ListMyIssues() (assigned, created, mentioned []messages.GitLabIssue, err error) {
	seenAssigned := make(map[int64]bool)
	seenCreated := make(map[int64]bool)

	// Fetch assigned issues for all tracked users.
	for _, username := range c.Usernames {
		opts := &gitlab.ListProjectIssuesOptions{
			State:            gitlab.Ptr("opened"),
			ListOptions:      gitlab.ListOptions{PerPage: 50},
			AssigneeUsername: gitlab.Ptr(username),
		}
		raw, _, err := c.Raw.Issues.ListProjectIssues(c.ProjectID, opts)
		if err != nil {
			continue
		}
		for _, issue := range convertIssues(raw) {
			if !seenAssigned[issue.IID] {
				seenAssigned[issue.IID] = true
				assigned = append(assigned, issue)
			}
		}
	}

	// Fetch created issues for all tracked users (skip if already in assigned).
	for _, uid := range c.UserIDs {
		opts := &gitlab.ListProjectIssuesOptions{
			State:       gitlab.Ptr("opened"),
			ListOptions: gitlab.ListOptions{PerPage: 50},
			AuthorID:    gitlab.Ptr(uid),
		}
		raw, _, err := c.Raw.Issues.ListProjectIssues(c.ProjectID, opts)
		if err != nil {
			continue
		}
		for _, issue := range convertIssues(raw) {
			if !seenAssigned[issue.IID] && !seenCreated[issue.IID] {
				seenCreated[issue.IID] = true
				created = append(created, issue)
			}
		}
	}

	return assigned, created, nil, nil
}

// GetIssue returns a single issue with its notes.
func (c *Client) GetIssue(iid int64) (messages.GitLabIssue, []messages.GitLabNote, error) {
	issue, _, err := c.Raw.Issues.GetIssue(c.ProjectID, iid)
	if err != nil {
		return messages.GitLabIssue{}, nil, fmt.Errorf("getting issue: %w", err)
	}

	notesRaw, _, err := c.Raw.Notes.ListIssueNotes(c.ProjectID, iid, &gitlab.ListIssueNotesOptions{
		ListOptions: gitlab.ListOptions{PerPage: 50},
		Sort:        gitlab.Ptr("asc"),
	})
	if err != nil {
		return convertIssue(issue), nil, nil // notes are non-critical
	}

	notes := make([]messages.GitLabNote, 0, len(notesRaw))
	for _, n := range notesRaw {
		if n.System {
			continue // skip system notes
		}
		notes = append(notes, messages.GitLabNote{
			Author:    n.Author.Username,
			Body:      n.Body,
			CreatedAt: safeTime(n.CreatedAt),
		})
	}

	return convertIssue(issue), notes, nil
}

// CloseIssue closes an issue.
func (c *Client) CloseIssue(iid int64) error {
	_, _, err := c.Raw.Issues.UpdateIssue(c.ProjectID, iid, &gitlab.UpdateIssueOptions{
		StateEvent: gitlab.Ptr("close"),
	})
	return err
}

// ReopenIssue reopens a closed issue.
func (c *Client) ReopenIssue(iid int64) error {
	_, _, err := c.Raw.Issues.UpdateIssue(c.ProjectID, iid, &gitlab.UpdateIssueOptions{
		StateEvent: gitlab.Ptr("reopen"),
	})
	return err
}

// CommentOnIssue adds a note to an issue.
func (c *Client) CommentOnIssue(iid int64, body string) error {
	_, _, err := c.Raw.Notes.CreateIssueNote(c.ProjectID, iid, &gitlab.CreateIssueNoteOptions{
		Body: gitlab.Ptr(body),
	})
	return err
}

// AssignIssue assigns a user to an issue.
func (c *Client) AssignIssue(iid int64, userID int64) error {
	_, _, err := c.Raw.Issues.UpdateIssue(c.ProjectID, iid, &gitlab.UpdateIssueOptions{
		AssigneeIDs: gitlab.Ptr([]int64{userID}),
	})
	return err
}

// SetIssueLabels sets labels on an issue.
func (c *Client) SetIssueLabels(iid int64, labels []string) error {
	lbls := gitlab.LabelOptions(labels)
	_, _, err := c.Raw.Issues.UpdateIssue(c.ProjectID, iid, &gitlab.UpdateIssueOptions{
		Labels: &lbls,
	})
	return err
}

// CreateIssue creates a new issue with the given title and description.
func (c *Client) CreateIssue(title, description string) (*messages.GitLabIssue, error) {
	issue, _, err := c.Raw.Issues.CreateIssue(c.ProjectID, &gitlab.CreateIssueOptions{
		Title:       gitlab.Ptr(title),
		Description: gitlab.Ptr(description),
	})
	if err != nil {
		return nil, err
	}
	result := convertIssue(issue)
	return &result, nil
}

func convertIssues(raw []*gitlab.Issue) []messages.GitLabIssue {
	result := make([]messages.GitLabIssue, len(raw))
	for i, issue := range raw {
		result[i] = convertIssue(issue)
	}
	return result
}

func convertIssue(issue *gitlab.Issue) messages.GitLabIssue {
	var assignee string
	if issue.Assignee != nil {
		assignee = issue.Assignee.Username
	}
	var labels []string
	for _, l := range issue.Labels {
		labels = append(labels, l)
	}
	var milestone string
	if issue.Milestone != nil {
		milestone = issue.Milestone.Title
	}
	return messages.GitLabIssue{
		ID:          issue.ID,
		IID:         issue.IID,
		ProjectID:   issue.ProjectID,
		Title:       issue.Title,
		State:       issue.State,
		Description: issue.Description,
		Labels:      labels,
		Milestone:   milestone,
		Author:      issue.Author.Username,
		Assignee:    assignee,
		WebURL:      issue.WebURL,
		CreatedAt:   safeTime(issue.CreatedAt),
		UpdatedAt:   safeTime(issue.UpdatedAt),
	}
}

// FormatIssueDetail formats an issue and its notes for the detail pane.
func FormatIssueDetail(issue messages.GitLabIssue, notes []messages.GitLabNote) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("#%d %s [%s]\n", issue.IID, issue.Title, issue.State))
	b.WriteString(strings.Repeat("─", 60) + "\n")

	if issue.Assignee != "" {
		b.WriteString(fmt.Sprintf("Assignee:  %s\n", issue.Assignee))
	}
	b.WriteString(fmt.Sprintf("Author:    %s\n", issue.Author))
	if len(issue.Labels) > 0 {
		b.WriteString(fmt.Sprintf("Labels:    %s\n", strings.Join(issue.Labels, ", ")))
	}
	if issue.Milestone != "" {
		b.WriteString(fmt.Sprintf("Milestone: %s\n", issue.Milestone))
	}
	b.WriteString(fmt.Sprintf("Created:   %s\n", issue.CreatedAt.Format("2006-01-02 15:04")))
	b.WriteString(fmt.Sprintf("Updated:   %s\n", issue.UpdatedAt.Format("2006-01-02 15:04")))
	b.WriteString(fmt.Sprintf("URL:       %s\n", issue.WebURL))

	if issue.Description != "" {
		b.WriteString("\n" + strings.Repeat("─", 60) + "\n")
		b.WriteString(issue.Description)
		b.WriteString("\n")
	}

	if len(notes) > 0 {
		b.WriteString("\n" + strings.Repeat("─", 60) + "\n")
		b.WriteString("COMMENTS\n")
		b.WriteString(strings.Repeat("─", 60) + "\n")
		for _, note := range notes {
			b.WriteString(fmt.Sprintf("\n@%s  %s\n", note.Author, note.CreatedAt.Format("2006-01-02 15:04")))
			b.WriteString(note.Body)
			b.WriteString("\n")
		}
	}

	return b.String()
}

func safeTime(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}
