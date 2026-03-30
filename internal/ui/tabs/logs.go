package tabs

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/abhishek-rana/lazydk/internal/ui"
	"github.com/abhishek-rana/lazydk/internal/ui/components"
	"github.com/abhishek-rana/lazydk/pkg/messages"
)

// LogsTab shows a merged log stream from all active Docker and K8s sources.
type LogsTab struct {
	logView components.LogView
	width   int
	height  int
}

// NewLogsTab creates a new merged logs tab.
func NewLogsTab() *LogsTab {
	lv := components.NewLogView()
	lv.SetFocused(true)

	return &LogsTab{
		logView: lv,
	}
}

func (t *LogsTab) Title() string { return "All Logs" }

func (t *LogsTab) SetSize(width, height int) {
	t.width = width
	t.height = height
	t.logView.SetSize(width, height)
}

func (t *LogsTab) Init() tea.Cmd {
	return nil
}

func (t *LogsTab) Update(msg tea.Msg) (ui.TabModel, tea.Cmd) {
	switch msg := msg.(type) {
	case messages.LogBatchMsg:
		// Accept log batches from ALL sources (Docker and K8s).
		t.logView.AppendLines(msg.Lines)
		return t, nil

	case tea.KeyPressMsg:
		cmd := t.logView.Update(msg)
		return t, cmd

	case logsRefreshTickMsg:
		return t, t.tickRefresh()
	}

	return t, nil
}

func (t *LogsTab) View() string {
	return t.logView.View()
}

// Notification implements the Notifier interface.
func (t *LogsTab) Notification() string {
	return ""
}

type logsRefreshTickMsg struct{}

func (t *LogsTab) tickRefresh() tea.Cmd {
	return tea.Tick(5*time.Second, func(time.Time) tea.Msg {
		return logsRefreshTickMsg{}
	})
}
