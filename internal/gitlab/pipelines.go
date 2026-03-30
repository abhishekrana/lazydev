package gitlab

import (
	"fmt"
	"io"
	"strings"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	"github.com/abhishek-rana/lazydev/pkg/messages"
)

// ListMyPipelines returns MR pipelines triggered by the tracked users,
// filtered to only merge-request refs and deduped to latest per MR+type.
func (c *Client) ListMyPipelines() ([]messages.GitLabPipeline, error) {
	seen := make(map[int64]bool)
	var all []messages.GitLabPipeline

	for _, username := range c.Usernames {
		opts := &gitlab.ListProjectPipelinesOptions{
			ListOptions: gitlab.ListOptions{PerPage: 50},
			Username:    gitlab.Ptr(username),
		}
		raw, _, err := c.Raw.Pipelines.ListProjectPipelines(c.ProjectID, opts)
		if err != nil {
			continue
		}
		for _, p := range convertPipelines(raw) {
			if !seen[p.ID] {
				seen[p.ID] = true
				all = append(all, p)
			}
		}
	}

	// Filter to MR pipelines only and dedup to latest per MR+type.
	dedupKey := make(map[string]bool)
	var result []messages.GitLabPipeline
	for _, p := range all {
		if p.MRIid == "" {
			continue
		}
		key := p.MRIid + "/" + p.PipelineType
		if !dedupKey[key] {
			dedupKey[key] = true
			result = append(result, p)
		}
	}

	return result, nil
}

// GetPipelineJobs returns the jobs for a pipeline.
func (c *Client) GetPipelineJobs(pipelineID int64) ([]messages.GitLabJob, error) {
	jobs, _, err := c.Raw.Jobs.ListPipelineJobs(c.ProjectID, pipelineID, &gitlab.ListJobsOptions{
		ListOptions: gitlab.ListOptions{PerPage: 100},
	})
	if err != nil {
		return nil, fmt.Errorf("listing pipeline jobs: %w", err)
	}

	result := make([]messages.GitLabJob, len(jobs))
	for i, j := range jobs {
		result[i] = messages.GitLabJob{
			ID:       j.ID,
			Name:     j.Name,
			Stage:    j.Stage,
			Status:   j.Status,
			Duration: j.Duration,
			WebURL:   j.WebURL,
		}
	}
	return result, nil
}

// GetJobLog returns the log trace for a job.
func (c *Client) GetJobLog(jobID int64) (string, error) {
	reader, _, err := c.Raw.Jobs.GetTraceFile(c.ProjectID, jobID)
	if err != nil {
		return "", fmt.Errorf("getting job log: %w", err)
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("reading job log: %w", err)
	}
	return string(data), nil
}

// RetryPipeline retries a failed pipeline.
func (c *Client) RetryPipeline(pipelineID int64) error {
	_, _, err := c.Raw.Pipelines.RetryPipelineBuild(c.ProjectID, pipelineID)
	return err
}

// CancelPipeline cancels a running pipeline.
func (c *Client) CancelPipeline(pipelineID int64) error {
	_, _, err := c.Raw.Pipelines.CancelPipelineBuild(c.ProjectID, pipelineID)
	return err
}

func convertPipelines(raw []*gitlab.PipelineInfo) []messages.GitLabPipeline {
	result := make([]messages.GitLabPipeline, 0, len(raw))
	for _, p := range raw {
		mrIID, pipelineType, _ := parseMRRef(p.Ref)
		result = append(result, messages.GitLabPipeline{
			ID:           p.ID,
			Status:       p.Status,
			Ref:          p.Ref,
			SHA:          p.SHA,
			WebURL:       p.WebURL,
			CreatedAt:    safeTime(p.CreatedAt),
			MRIid:        mrIID,
			PipelineType: pipelineType,
		})
	}
	return result
}

// parseMRRef extracts MR IID and pipeline type from a merge request ref.
// Returns ("1353", "merge", true) for "refs/merge-requests/1353/merge".
// Returns ("", "", false) for non-MR refs like "main".
func parseMRRef(ref string) (mrIID string, pipelineType string, ok bool) {
	const prefix = "refs/merge-requests/"
	if !strings.HasPrefix(ref, prefix) {
		return "", "", false
	}
	rest := ref[len(prefix):]
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) != 2 || parts[0] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// FormatPipelineDetail formats a pipeline and its jobs for the detail pane.
func FormatPipelineDetail(pipeline messages.GitLabPipeline, jobs []messages.GitLabJob) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Pipeline #%d [%s]\n", pipeline.ID, pipeline.Status)
	b.WriteString(strings.Repeat("─", 60) + "\n")

	fmt.Fprintf(&b, "Ref:     %s\n", pipeline.Ref)
	if pipeline.MRIid != "" {
		fmt.Fprintf(&b, "MR:      !%s [%s]\n", pipeline.MRIid, pipeline.PipelineType)
	}
	if pipeline.SHA != "" {
		sha := pipeline.SHA
		if len(sha) > 8 {
			sha = sha[:8]
		}
		fmt.Fprintf(&b, "SHA:     %s\n", sha)
	}
	if pipeline.Duration > 0 {
		fmt.Fprintf(&b, "Duration: %.0fs\n", pipeline.Duration)
	}
	if !pipeline.CreatedAt.IsZero() {
		fmt.Fprintf(&b, "Created: %s\n", pipeline.CreatedAt.Format("2006-01-02 15:04"))
	}
	if !pipeline.FinishedAt.IsZero() {
		fmt.Fprintf(&b, "Finished: %s\n", pipeline.FinishedAt.Format("2006-01-02 15:04"))
	}
	fmt.Fprintf(&b, "URL:     %s\n", pipeline.WebURL)

	if len(jobs) > 0 {
		b.WriteString("\n" + strings.Repeat("─", 60) + "\n")
		b.WriteString("JOBS\n")
		b.WriteString(strings.Repeat("─", 60) + "\n\n")

		currentStage := ""
		for _, job := range jobs {
			if job.Stage != currentStage {
				currentStage = job.Stage
				fmt.Fprintf(&b, "  [%s]\n", currentStage)
			}
			icon := jobStatusIcon(job.Status)
			dur := ""
			if job.Duration > 0 {
				dur = fmt.Sprintf(" (%.0fs)", job.Duration)
			}
			fmt.Fprintf(&b, "    %s %s%s\n", icon, job.Name, dur)
		}
	}

	return b.String()
}

// PipelineStatusIcon returns a status icon for a pipeline status string.
func PipelineStatusIcon(status string) string {
	return jobStatusIcon(status)
}

func jobStatusIcon(status string) string {
	switch status {
	case "success":
		return "✓"
	case "failed":
		return "✗"
	case "running":
		return "◌"
	case "pending":
		return "○"
	case "canceled":
		return "⊘"
	case "skipped":
		return "→"
	default:
		return "?"
	}
}
