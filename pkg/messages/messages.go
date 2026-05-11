package messages

import (
	"time"
)

// --- Sidebar generic item ---

// SidebarItem is a generic row in the sidebar. Both Issues and MRs map onto it.
type SidebarItem struct {
	ID    string
	Name  string
	State ItemState
	Group string
}

// ItemState is the colored-icon state used by the sidebar.
type ItemState int

const (
	StateUnknown ItemState = iota
	StateOpen              // green ●
	StateClosed            // grey ○
	StateMerged            // red ✗ (reused color slot, semantics differ for MRs)
	StateDraft             // yellow ◌
)

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

// GitLabIteration represents a GitLab iteration (sprint).
type GitLabIteration struct {
	ID    int64
	Title string
	Start time.Time
	Due   time.Time
}

// --- GitLab messages ---

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

// --- Cache + sync messages ---

// CacheUpdatedMsg signals new cache contents are available for a kind.
type CacheUpdatedMsg struct {
	Kind string // "issues" | "mrs"
}

// SyncStatusMsg reports the syncer's current state.
type SyncStatusMsg struct {
	State      string // "prefetching" | "syncing" | "idle" | "offline"
	Progress   string // e.g. "120/450"
	LastSyncAt time.Time
	Err        error
}

// --- Selection + export messages ---

// SelectionChangedMsg signals the multi-select set changed.
type SelectionChangedMsg struct {
	Count int
}

// ExportDoneMsg reports the result of a context export.
type ExportDoneMsg struct {
	Channel string // "clipboard" | "file" | "pipe"
	Path    string // for "file" channel
	Items   int
	Err     error
}

// --- Saved view + query messages ---

// ApplyViewMsg requests that the active tab apply the given DSL
// expression as its current filter (typically from a number-key
// view recall or from the command palette).
type ApplyViewMsg struct {
	Name string
	Expr string
}

// --- General UI messages ---

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

// ExecFinishedMsg is sent when an exec-style external process completes
// (e.g. the user's $EDITOR finishing a comment draft).
type ExecFinishedMsg struct {
	Err error
}
