package tabs

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/abhishek-rana/lazydk/internal/ui"
	"github.com/abhishek-rana/lazydk/internal/ui/components"
	"github.com/abhishek-rana/lazydk/pkg/messages"
)

// LogsTab shows a merged log stream from all active Docker and K8s sources.
type LogsTab struct {
	logView      components.LogView
	width        int
	height       int
	notification string
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
	t.logView.SetOffset(0, 2) // tab bar height
	t.logView.SetFocused(true) // always focused in this tab
}

func (t *LogsTab) Init() tea.Cmd {
	return nil
}

func (t *LogsTab) Update(msg tea.Msg) (ui.TabModel, tea.Cmd) {
	switch msg := msg.(type) {
	case messages.LogBatchMsg:
		t.logView.AppendLines(msg.Lines)
		return t, nil

	case messages.LogExportedMsg:
		if msg.Err == nil {
			t.notification = fmt.Sprintf("exported to %s", msg.Path)
		} else {
			t.notification = fmt.Sprintf("export failed: %v", msg.Err)
		}
		return t, nil

	case tea.MouseClickMsg, tea.MouseWheelMsg:
		t.logView.SetFocused(true)
		cmd := t.logView.Update(msg)
		return t, cmd

	case tea.KeyPressMsg:
		// Ensure logview stays focused.
		t.logView.SetFocused(true)
		cmd := t.logView.Update(msg)
		return t, cmd
	}

	return t, nil
}

func (t *LogsTab) View() string {
	return t.logView.View()
}

// Notification implements the Notifier interface.
func (t *LogsTab) Notification() string {
	n := t.notification
	t.notification = "" // clear after reading
	return n
}
