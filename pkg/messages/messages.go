package messages

import (
	"time"
)

// LogLine represents a single log line from any source.
type LogLine struct {
	Source   string
	SourceID string
	Text     string
	Level    LogLevel
	Time     time.Time
}

// LogLevel represents the severity of a log line.
type LogLevel int

const (
	LogLevelUnknown LogLevel = iota
	LogLevelDebug
	LogLevelInfo
	LogLevelWarn
	LogLevelError
	LogLevelFatal
)

// Container represents a generic container/pod resource.
type Container struct {
	ID       string
	Name     string
	Status   string
	State    ContainerState
	Source   string // "docker" or "kubernetes"
	Group    string // compose project or k8s namespace
	Image    string
	Created  time.Time
	Restarts int
}

// ContainerState represents the running state.
type ContainerState int

const (
	StateUnknown ContainerState = iota
	StateRunning
	StateStopped
	StateError
	StatePending
	StateRestarting
)

// --- Bubble Tea Messages ---

// ContainerListMsg is sent when a container list is fetched.
type ContainerListMsg struct {
	Containers []Container
	Source     string // "docker" or "kubernetes"
	Err        error
}

// LogBatchMsg delivers a batch of log lines to the UI.
type LogBatchMsg struct {
	Lines    []LogLine
	SourceID string
}

// LogStreamStartedMsg confirms a log stream was started.
type LogStreamStartedMsg struct {
	SourceID string
}

// LogStreamErrorMsg reports a log stream error.
type LogStreamErrorMsg struct {
	SourceID string
	Err      error
}

// ContainerActionMsg reports the result of a container action.
type ContainerActionMsg struct {
	Action string // "restart", "stop", "remove"
	ID     string
	Name   string
	Err    error
}

// ContainerInspectMsg delivers container inspect/describe data.
type ContainerInspectMsg struct {
	ID   string
	Data string // formatted YAML/JSON
	Err  error
}

// ResourceStats holds CPU/memory stats for a container or pod.
type ResourceStats struct {
	ID     string
	Name   string
	Source string // "docker" or "kubernetes"
	CPU    string // e.g. "12.5%" or "100m"
	Memory string // e.g. "45.2 MiB" or "128Mi"
}

// ResourceStatsMsg delivers stats for multiple resources.
type ResourceStatsMsg struct {
	Stats  []ResourceStats
	Source string
	Err    error
}

// DashboardRow represents a single row in the dashboard table.
type DashboardRow struct {
	Name     string
	Type     string // "container" or "pod"
	Source   string // "docker" or "kubernetes"
	Group    string // compose project or namespace
	Status   string
	State    ContainerState
	Restarts int
	CPU      string
	Memory   string
}

// ExecFinishedMsg is sent when an exec shell session completes.
type ExecFinishedMsg struct {
	Err error
}

// PortForwardStartedMsg is sent when a port-forward starts.
type PortForwardStartedMsg struct {
	Namespace  string
	Pod        string
	LocalPort  string
	RemotePort string
}

// PortForwardStoppedMsg is sent when a port-forward stops.
type PortForwardStoppedMsg struct {
	Pod string
	Err error
}

// ScaleMsg reports the result of a scale operation.
type ScaleMsg struct {
	Name     string
	Replicas int
	Err      error
}

// LogExportedMsg is sent when logs are exported to a file.
type LogExportedMsg struct {
	Path string
	Err  error
}

// ClipboardMsg is sent when content is copied to clipboard.
type ClipboardMsg struct {
	Lines int
	Err   error
}

// DiscoveryResultMsg reports which backends are available.
type DiscoveryResultMsg struct {
	DockerAvailable bool
	DockerHost      string
	KubeAvailable   bool
	KubeContext     string
	GitLabAvailable bool
	GitLabProject   string
	Warnings        []string
}

// --- GitLab data types ---

// GitLabIssue represents a GitLab issue.
type GitLabIssue struct {
	ID, IID, ProjectID int64
	Title              string
	State              string
	Description        string
	Labels             []string
	Milestone          string
	IterationID        int64  // iteration ID for matching
	Iteration          string // iteration title (e.g. "Sprint 5")
	IterationDates     string // e.g. "Mar 22 – Apr 4, 2026"
	Author             string
	Assignee           string
	WebURL             string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// GitLabMR represents a GitLab merge request.
type GitLabMR struct {
	ID, IID, ProjectID int64
	Title              string
	State              string
	Description        string
	SourceBranch       string
	TargetBranch       string
	Author             string
	Assignee           string
	Reviewers          []string
	Labels             []string
	PipelineStatus     string
	ChangesCount       string
	WebURL             string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// GitLabPipeline represents a GitLab CI pipeline.
type GitLabPipeline struct {
	ID, ProjectID int64
	Status        string
	Ref           string
	SHA           string
	WebURL        string
	Duration      float64
	CreatedAt     time.Time
	FinishedAt    time.Time
}

// GitLabJob represents a job within a pipeline.
type GitLabJob struct {
	ID       int64
	Name     string
	Stage    string
	Status   string
	Duration float64
	WebURL   string
}

// GitLabIssueMR represents a merge request linked to an issue.
type GitLabIssueMR struct {
	IID          int64
	Title        string
	State        string
	SourceBranch string
	WebURL       string
}

// GitLabNote represents a comment on an issue or MR.
type GitLabNote struct {
	Author    string
	Body      string
	CreatedAt time.Time
}

// --- GitLab messages ---

// GitLabIteration represents a GitLab iteration (sprint).
type GitLabIteration struct {
	ID    int64
	Title string
	Start time.Time
	Due   time.Time
}

// IssueListMsg delivers issue lists from GitLab.
type IssueListMsg struct {
	Assigned         []GitLabIssue
	Created          []GitLabIssue
	Mentioned        []GitLabIssue
	CurrentIteration *GitLabIteration // active iteration, if any
	Err              error
}

// IssueDetailMsg delivers a single issue with notes and related MRs.
type IssueDetailMsg struct {
	Issue      GitLabIssue
	Notes      []GitLabNote
	RelatedMRs []GitLabIssueMR
	Err        error
}

// IssueActionMsg reports the result of an issue action.
type IssueActionMsg struct {
	Action string
	Err    error
}

// MRListMsg delivers merge request lists from GitLab.
type MRListMsg struct {
	Mine            []GitLabMR
	ReviewRequested []GitLabMR
	AllOpen         []GitLabMR
	Err             error
}

// MRDetailMsg delivers a single MR with notes.
type MRDetailMsg struct {
	MR    GitLabMR
	Notes []GitLabNote
	Err   error
}

// MRActionMsg reports the result of a MR action.
type MRActionMsg struct {
	Action string
	Err    error
}

// PipelineListMsg delivers pipeline lists from GitLab.
type PipelineListMsg struct {
	Mine []GitLabPipeline
	All  []GitLabPipeline
	Err  error
}

// PipelineJobsMsg delivers jobs for a pipeline.
type PipelineJobsMsg struct {
	PipelineID int64
	Jobs       []GitLabJob
	Err        error
}

// JobLogMsg delivers log lines for a pipeline job.
type JobLogMsg struct {
	JobID int64
	Log   string
	Err   error
}

// SwitchTabMsg requests switching to a specific tab.
type SwitchTabMsg struct {
	Tab int
}

// ShowModalMsg shows a modal dialog.
type ShowModalMsg struct {
	Title   string
	Message string
	OnOK    func() // called if user confirms
}

// DismissModalMsg dismisses the current modal.
type DismissModalMsg struct{}

// ErrorMsg is a generic error notification.
type ErrorMsg struct {
	Err error
}

// TickMsg is sent periodically for refreshing data.
type TickMsg time.Time

// WindowSizeMsg is re-exported for convenience.
type WindowSizeMsg struct {
	Width  int
	Height int
}

// TabActivatedMsg is sent to a tab when it becomes the active tab.
type TabActivatedMsg struct{}
