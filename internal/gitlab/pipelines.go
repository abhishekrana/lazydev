package gitlab

import (
	"fmt"
	"io"
	"strings"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	"github.com/abhishek-rana/lazydev/pkg/messages"
)

// ListMyPipelines returns pipelines triggered by the authenticated user and all recent pipelines.
func (c *Client) ListMyPipelines() (mine, all []messages.GitLabPipeline, err error) {
	seen := make(map[int64]bool)

	// My pipelines (from all tracked users).
	for _, username := range c.Usernames {
		opts := &gitlab.ListProjectPipelinesOptions{
			ListOptions: gitlab.ListOptions{PerPage: 30},
			Username:    gitlab.Ptr(username),
		}
		raw, _, err := c.Raw.Pipelines.ListProjectPipelines(c.ProjectID, opts)
		if err != nil {
			continue
		}
		for _, p := range convertPipelines(raw) {
			if !seen[p.ID] {
				seen[p.ID] = true
				mine = append(mine, p)
			}
		}
	}

	// All pipelines.
	allRaw, _, err := c.Raw.Pipelines.ListProjectPipelines(c.ProjectID, &gitlab.ListProjectPipelinesOptions{
		ListOptions: gitlab.ListOptions{PerPage: 30},
	})
	if err == nil {
		all = convertPipelines(allRaw)
	}

	return mine, all, nil
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
	result := make([]messages.GitLabPipeline, len(raw))
	for i, p := range raw {
		result[i] = messages.GitLabPipeline{
			ID:        p.ID,
			Status:    p.Status,
			Ref:       p.Ref,
			SHA:       p.SHA,
			WebURL:    p.WebURL,
			CreatedAt: safeTime(p.CreatedAt),
		}
	}
	return result
}

// FormatPipelineDetail formats a pipeline and its jobs for the detail pane.
func FormatPipelineDetail(pipeline messages.GitLabPipeline, jobs []messages.GitLabJob) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Pipeline #%d [%s]\n", pipeline.ID, pipeline.Status))
	b.WriteString(strings.Repeat("─", 60) + "\n")

	b.WriteString(fmt.Sprintf("Ref:     %s\n", pipeline.Ref))
	if pipeline.SHA != "" {
		sha := pipeline.SHA
		if len(sha) > 8 {
			sha = sha[:8]
		}
		b.WriteString(fmt.Sprintf("SHA:     %s\n", sha))
	}
	if pipeline.Duration > 0 {
		b.WriteString(fmt.Sprintf("Duration: %.0fs\n", pipeline.Duration))
	}
	if !pipeline.CreatedAt.IsZero() {
		b.WriteString(fmt.Sprintf("Created: %s\n", pipeline.CreatedAt.Format("2006-01-02 15:04")))
	}
	if !pipeline.FinishedAt.IsZero() {
		b.WriteString(fmt.Sprintf("Finished: %s\n", pipeline.FinishedAt.Format("2006-01-02 15:04")))
	}
	b.WriteString(fmt.Sprintf("URL:     %s\n", pipeline.WebURL))

	if len(jobs) > 0 {
		b.WriteString("\n" + strings.Repeat("─", 60) + "\n")
		b.WriteString("JOBS\n")
		b.WriteString(strings.Repeat("─", 60) + "\n\n")

		currentStage := ""
		for _, job := range jobs {
			if job.Stage != currentStage {
				currentStage = job.Stage
				b.WriteString(fmt.Sprintf("  [%s]\n", currentStage))
			}
			icon := jobStatusIcon(job.Status)
			dur := ""
			if job.Duration > 0 {
				dur = fmt.Sprintf(" (%.0fs)", job.Duration)
			}
			b.WriteString(fmt.Sprintf("    %s %s%s\n", icon, job.Name, dur))
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
