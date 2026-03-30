package messages

import (
	"time"
)

// LogLine represents a single log line from any source.
type LogLine struct {
	Source    string
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
	ID        string
	Name      string
	Status    string
	State     ContainerState
	Source    string // "docker" or "kubernetes"
	Group     string // compose project or k8s namespace
	Image     string
	Created   time.Time
	Restarts  int
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
	Action   string // "restart", "stop", "remove"
	ID       string
	Name     string
	Err      error
}

// ContainerInspectMsg delivers container inspect/describe data.
type ContainerInspectMsg struct {
	ID   string
	Data string // formatted YAML/JSON
	Err  error
}

// DiscoveryResultMsg reports which backends are available.
type DiscoveryResultMsg struct {
	DockerAvailable bool
	DockerHost      string
	KubeAvailable   bool
	KubeContext     string
	Warnings        []string
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
