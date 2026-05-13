package gitlab

import (
	"fmt"
	"strings"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	"github.com/abhishek-rana/lazydev/pkg/messages"
)

// ListMyMRs returns merge requests authored by, reviewing, or all open.
func (c *Client) ListMyMRs() (mine, reviewRequested, allOpen []messages.GitLabMR, err error) {
	seenMine := make(map[int64]bool)
	seenReview := make(map[int64]bool)

	// My MRs (authored by any tracked user).
	for _, username := range c.Usernames {
		opts := &gitlab.ListProjectMergeRequestsOptions{
			State:          gitlab.Ptr("opened"),
			ListOptions:    gitlab.ListOptions{PerPage: 50},
			AuthorUsername: gitlab.Ptr(username),
		}
		raw, _, err := c.Raw.MergeRequests.ListProjectMergeRequests(c.ProjectID, opts)
		if err != nil {
			continue
		}
		for _, mr := range convertMRs(raw) {
			if !seenMine[mr.IID] {
				seenMine[mr.IID] = true
				mine = append(mine, mr)
			}
		}
	}

	// Review requested (for any tracked user).
	for _, username := range c.Usernames {
		opts := &gitlab.ListProjectMergeRequestsOptions{
			State:            gitlab.Ptr("opened"),
			ListOptions:      gitlab.ListOptions{PerPage: 50},
			ReviewerUsername: gitlab.Ptr(username),
		}
		raw, _, err := c.Raw.MergeRequests.ListProjectMergeRequests(c.ProjectID, opts)
		if err != nil {
			continue
		}
		for _, mr := range convertMRs(raw) {
			if !seenMine[mr.IID] && !seenReview[mr.IID] {
				seenReview[mr.IID] = true
				reviewRequested = append(reviewRequested, mr)
			}
		}
	}

	// All open.
	allRaw, _, err := c.Raw.MergeRequests.ListProjectMergeRequests(c.ProjectID, &gitlab.ListProjectMergeRequestsOptions{
		State:       gitlab.Ptr("opened"),
		ListOptions: gitlab.ListOptions{PerPage: 50},
	})
	if err == nil {
		allOpen = convertMRs(allRaw)
	}

	return mine, reviewRequested, allOpen, nil
}

// GetMR returns a single merge request with its notes.
func (c *Client) GetMR(iid int64) (messages.GitLabMR, []messages.GitLabNote, error) {
	mr, _, err := c.Raw.MergeRequests.GetMergeRequest(c.ProjectID, iid, &gitlab.GetMergeRequestsOptions{})
	if err != nil {
		return messages.GitLabMR{}, nil, fmt.Errorf("getting MR: %w", err)
	}

	notesRaw, _, err := c.Raw.Notes.ListMergeRequestNotes(c.ProjectID, iid, &gitlab.ListMergeRequestNotesOptions{
		ListOptions: gitlab.ListOptions{PerPage: 50},
		Sort:        gitlab.Ptr("asc"),
	})
	if err != nil {
		return convertFullMR(mr), nil, nil
	}

	notes := make([]messages.GitLabNote, 0, len(notesRaw))
	for _, n := range notesRaw {
		if n.System {
			continue
		}
		notes = append(notes, messages.GitLabNote{
			Author:    n.Author.Username,
			Body:      n.Body,
			CreatedAt: safeTime(n.CreatedAt),
		})
	}

	return convertFullMR(mr), notes, nil
}

// AssignMR sets the assignee on a merge request.
func (c *Client) AssignMR(iid int64, userID int64) error {
	_, _, err := c.Raw.MergeRequests.UpdateMergeRequest(c.ProjectID, iid, &gitlab.UpdateMergeRequestOptions{
		AssigneeIDs: gitlab.Ptr([]int64{userID}),
	})
	return err
}

// ApproveMR approves a merge request.
func (c *Client) ApproveMR(iid int64) error {
	_, _, err := c.Raw.MergeRequestApprovals.ApproveMergeRequest(c.ProjectID, iid, &gitlab.ApproveMergeRequestOptions{})
	return err
}

// MergeMR merges a merge request.
func (c *Client) MergeMR(iid int64) error {
	_, _, err := c.Raw.MergeRequests.AcceptMergeRequest(c.ProjectID, iid, &gitlab.AcceptMergeRequestOptions{})
	return err
}

// CloseMR closes a merge request.
func (c *Client) CloseMR(iid int64) error {
	_, _, err := c.Raw.MergeRequests.UpdateMergeRequest(c.ProjectID, iid, &gitlab.UpdateMergeRequestOptions{
		StateEvent: gitlab.Ptr("close"),
	})
	return err
}

// ReopenMR reopens a merge request.
func (c *Client) ReopenMR(iid int64) error {
	_, _, err := c.Raw.MergeRequests.UpdateMergeRequest(c.ProjectID, iid, &gitlab.UpdateMergeRequestOptions{
		StateEvent: gitlab.Ptr("reopen"),
	})
	return err
}

// CommentOnMR adds a note to a merge request.
func (c *Client) CommentOnMR(iid int64, body string) error {
	_, _, err := c.Raw.Notes.CreateMergeRequestNote(c.ProjectID, iid, &gitlab.CreateMergeRequestNoteOptions{
		Body: gitlab.Ptr(body),
	})
	return err
}

func convertMRs(raw []*gitlab.BasicMergeRequest) []messages.GitLabMR {
	result := make([]messages.GitLabMR, len(raw))
	for i, mr := range raw {
		result[i] = convertBasicMR(mr)
	}
	return result
}

func convertBasicMR(mr *gitlab.BasicMergeRequest) messages.GitLabMR {
	var author string
	if mr.Author != nil {
		author = mr.Author.Username
	}
	labels := make([]string, 0, len(mr.Labels))
	for _, l := range mr.Labels {
		labels = append(labels, l)
	}
	return messages.GitLabMR{
		ID:           mr.ID,
		IID:          mr.IID,
		ProjectID:    mr.ProjectID,
		Title:        mr.Title,
		State:        mr.State,
		SourceBranch: mr.SourceBranch,
		TargetBranch: mr.TargetBranch,
		Author:       author,
		Labels:       labels,
		WebURL:       mr.WebURL,
		CreatedAt:    safeTime(mr.CreatedAt),
		UpdatedAt:    safeTime(mr.UpdatedAt),
	}
}

func convertFullMR(mr *gitlab.MergeRequest) messages.GitLabMR {
	reviewers := make([]string, 0, len(mr.Reviewers))
	for _, r := range mr.Reviewers {
		reviewers = append(reviewers, r.Username)
	}
	labels := make([]string, 0, len(mr.Labels))
	for _, l := range mr.Labels {
		labels = append(labels, l)
	}
	var pipelineStatus string
	if mr.HeadPipeline != nil {
		pipelineStatus = mr.HeadPipeline.Status
	} else if mr.Pipeline != nil {
		pipelineStatus = mr.Pipeline.Status
	}
	assignees := make([]string, 0, len(mr.Assignees))
	for _, a := range mr.Assignees {
		if a != nil {
			assignees = append(assignees, a.Username)
		}
	}
	return messages.GitLabMR{
		ID:             mr.ID,
		IID:            mr.IID,
		ProjectID:      mr.ProjectID,
		Title:          mr.Title,
		State:          mr.State,
		Description:    mr.Description,
		SourceBranch:   mr.SourceBranch,
		TargetBranch:   mr.TargetBranch,
		Author:         mr.Author.Username,
		Assignees:      assignees,
		Reviewers:      reviewers,
		Labels:         labels,
		PipelineStatus: pipelineStatus,
		ChangesCount:   mr.ChangesCount,
		WebURL:         mr.WebURL,
		CreatedAt:      safeTime(mr.CreatedAt),
		UpdatedAt:      safeTime(mr.UpdatedAt),
	}
}

// FormatMRDetail formats a merge request and its notes for the detail pane.
// width is used for word wrapping markdown content.
func FormatMRDetail(mr messages.GitLabMR, notes []messages.GitLabNote, width int) string {
	markdownWidth = width
	var b strings.Builder

	var pipeline string
	if mr.PipelineStatus != "" {
		pipeline = pipelineStatusIcon(mr.PipelineStatus) + " " + mr.PipelineStatus
	}
	var changes string
	if mr.ChangesCount != "" {
		changes = mr.ChangesCount + " files"
	}
	rows := []labeled{
		{"State", FormatState(mr.State)},
		{"Assignees", strings.Join(mr.Assignees, ", ")},
		{"Reviewers", strings.Join(mr.Reviewers, ", ")},
		{"Labels", strings.Join(mr.Labels, ", ")},
		{"Source", mr.SourceBranch},
		{"Target", mr.TargetBranch},
		{"Pipeline", pipeline},
		{"Changes", changes},
		{"Author", mr.Author},
		{"Updated", mr.UpdatedAt.Format("2006-01-02 15:04")},
		{"URL", mr.WebURL},
	}
	b.WriteString(formatHeaderStrip(rows, width))

	baseURL := projectBaseURL(mr.WebURL)
	hostURL := gitlabHostURL(mr.WebURL)

	if mr.Description != "" {
		b.WriteString("\n" + rule(width) + "\n")
		b.WriteString(renderMarkdown(mr.Description, baseURL, hostURL, mr.ProjectID))
	}

	if len(notes) > 0 {
		b.WriteString("\n" + rule(width) + "\n")
		fmt.Fprintf(&b, "Discussion (%d)\n", len(notes))
		for i, note := range notes {
			if i > 0 {
				b.WriteString("\n" + commentSep() + "\n")
			}
			fmt.Fprintf(&b, "\n@%s  %s\n\n", note.Author, note.CreatedAt.Format("2006-01-02 15:04"))
			b.WriteString(strings.TrimRight(renderMarkdown(note.Body, baseURL, hostURL, mr.ProjectID), "\n"))
			b.WriteByte('\n')
		}
	}

	return b.String()
}

// pipelineStatusIcon returns a single-glyph icon for a pipeline status.
func pipelineStatusIcon(status string) string {
	switch status {
	case "success":
		return "✓"
	case "failed":
		return "✗"
	case "running", "pending", "created", "preparing", "waiting_for_resource":
		return "◌"
	case "canceled", "skipped":
		return "⊘"
	case "manual":
		return "⚙"
	default:
		return "•"
	}
}
