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
	var assignee string
	if mr.Assignee != nil {
		assignee = mr.Assignee.Username
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
		Assignee:       assignee,
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

	fmt.Fprintf(&b, "!%d %s [%s]\n", mr.IID, mr.Title, mr.State)
	b.WriteString(strings.Repeat("─", 60) + "\n")

	fmt.Fprintf(&b, "Branch:    %s → %s\n", mr.SourceBranch, mr.TargetBranch)
	fmt.Fprintf(&b, "Author:    %s\n", mr.Author)
	if mr.Assignee != "" {
		fmt.Fprintf(&b, "Assignee:  %s\n", mr.Assignee)
	}
	if len(mr.Reviewers) > 0 {
		fmt.Fprintf(&b, "Reviewers: %s\n", strings.Join(mr.Reviewers, ", "))
	}
	if len(mr.Labels) > 0 {
		fmt.Fprintf(&b, "Labels:    %s\n", strings.Join(mr.Labels, ", "))
	}
	if mr.PipelineStatus != "" {
		icon := PipelineStatusIcon(mr.PipelineStatus)
		fmt.Fprintf(&b, "Pipeline:  %s %s\n", icon, mr.PipelineStatus)
	}
	if mr.ChangesCount != "" {
		fmt.Fprintf(&b, "Changes:   %s files\n", mr.ChangesCount)
	}
	fmt.Fprintf(&b, "Created:   %s\n", mr.CreatedAt.Format("2006-01-02 15:04"))
	fmt.Fprintf(&b, "Updated:   %s\n", mr.UpdatedAt.Format("2006-01-02 15:04"))
	fmt.Fprintf(&b, "URL:       %s\n", mr.WebURL)

	baseURL := projectBaseURL(mr.WebURL)
	hostURL := gitlabHostURL(mr.WebURL)

	if mr.Description != "" {
		b.WriteString("\n" + strings.Repeat("─", 60) + "\n")
		b.WriteString(renderMarkdown(mr.Description, baseURL, hostURL, mr.ProjectID))
	}

	if len(notes) > 0 {
		b.WriteString("\n" + strings.Repeat("─", 60) + "\n")
		b.WriteString("DISCUSSION\n")
		b.WriteString(strings.Repeat("─", 60) + "\n")
		for _, note := range notes {
			fmt.Fprintf(&b, "\n@%s  %s\n", note.Author, note.CreatedAt.Format("2006-01-02 15:04"))
			b.WriteString(renderMarkdown(note.Body, baseURL, hostURL, mr.ProjectID))
			b.WriteString("\n")
		}
	}

	return b.String()
}
