// Package gitlab — sync-oriented helpers used by the local cache to
// pull all activity in a project (not just items assigned to the
// authenticated user) since a given timestamp.
package gitlab

import (
	"time"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	"github.com/abhishek-rana/lazydev/pkg/messages"
)

// ListIssuesUpdatedAfter paginates project issues with updated_at > t,
// across all authors and assignees. Pass a zero time to fetch every
// issue (used for the first-run prefetch). State "" or "all" returns
// open + closed; pass "opened" or "closed" to narrow.
//
// Results are returned in ascending updated_at order so callers can
// upsert in batches and persist their high-water mark progressively.
func (c *Client) ListIssuesUpdatedAfter(t time.Time, state string) ([]messages.GitLabIssue, error) {
	var out []messages.GitLabIssue
	var page int64 = 1
	for {
		opts := &gitlab.ListProjectIssuesOptions{
			ListOptions: gitlab.ListOptions{Page: page, PerPage: 100},
			OrderBy:     gitlab.Ptr("updated_at"),
			Sort:        gitlab.Ptr("asc"),
		}
		if state != "" && state != "all" {
			opts.State = gitlab.Ptr(state)
		}
		if !t.IsZero() {
			opts.UpdatedAfter = &t
		}
		raw, resp, err := c.Raw.Issues.ListProjectIssues(c.ProjectID, opts)
		if err != nil {
			return out, err
		}
		out = append(out, convertIssues(raw)...)
		if resp == nil || resp.NextPage == 0 {
			return out, nil
		}
		page = resp.NextPage
	}
}

// ListMRsUpdatedAfter paginates project merge requests with
// updated_at > t. Same shape and semantics as ListIssuesUpdatedAfter.
// Returns MRs in ascending updated_at order. State filtering accepts
// "opened", "closed", "merged", "locked", "" / "all".
//
// Note: list responses give us BasicMergeRequest fields only — the MR
// Description IS included (see BasicMergeRequest), but the head pipeline
// status is not. PipelineStatus is populated on detail fetch via GetMR.
func (c *Client) ListMRsUpdatedAfter(t time.Time, state string) ([]messages.GitLabMR, error) {
	var out []messages.GitLabMR
	var page int64 = 1
	for {
		opts := &gitlab.ListProjectMergeRequestsOptions{
			ListOptions: gitlab.ListOptions{Page: page, PerPage: 100},
			OrderBy:     gitlab.Ptr("updated_at"),
			Sort:        gitlab.Ptr("asc"),
		}
		if state != "" && state != "all" {
			opts.State = gitlab.Ptr(state)
		}
		if !t.IsZero() {
			opts.UpdatedAfter = &t
		}
		raw, resp, err := c.Raw.MergeRequests.ListProjectMergeRequests(c.ProjectID, opts)
		if err != nil {
			return out, err
		}
		for _, mr := range raw {
			out = append(out, basicMRToFull(mr))
		}
		if resp == nil || resp.NextPage == 0 {
			return out, nil
		}
		page = resp.NextPage
	}
}

// basicMRToFull turns the lightweight list response into a
// messages.GitLabMR populated with every field present in
// BasicMergeRequest. PipelineStatus and ChangesCount remain empty —
// callers wanting those fields must GetMR by IID.
func basicMRToFull(mr *gitlab.BasicMergeRequest) messages.GitLabMR {
	var author, assignee string
	if mr.Author != nil {
		author = mr.Author.Username
	}
	if mr.Assignee != nil {
		assignee = mr.Assignee.Username
	}
	reviewers := make([]string, 0, len(mr.Reviewers))
	for _, r := range mr.Reviewers {
		if r != nil {
			reviewers = append(reviewers, r.Username)
		}
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
		Description:  mr.Description,
		SourceBranch: mr.SourceBranch,
		TargetBranch: mr.TargetBranch,
		Author:       author,
		Assignee:     assignee,
		Reviewers:    reviewers,
		Labels:       labels,
		WebURL:       mr.WebURL,
		CreatedAt:    safeTime(mr.CreatedAt),
		UpdatedAt:    safeTime(mr.UpdatedAt),
	}
}
