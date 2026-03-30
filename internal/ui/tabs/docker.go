package tabs

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/abhishek-rana/lazydk/internal/docker"
	logpkg "github.com/abhishek-rana/lazydk/internal/log"
	"github.com/abhishek-rana/lazydk/internal/ui"
	"github.com/abhishek-rana/lazydk/internal/ui/components"
	"github.com/abhishek-rana/lazydk/internal/ui/layout"
	"github.com/abhishek-rana/lazydk/internal/ui/theme"
	"github.com/abhishek-rana/lazydk/pkg/messages"
)

// paneMode tracks which pane is shown on the right.
type paneMode int

const (
	paneLogs paneMode = iota
	paneDetail
)

// DockerTab is the Docker container management tab.
type DockerTab struct {
	client       *docker.Client
	streamMgr    *logpkg.StreamManager
	sidebar      components.Sidebar
	logView      components.LogView
	detailPane   components.DetailPane
	modal        components.Modal
	focusSidebar bool
	rightPane    paneMode
	width        int
	height       int
	selected     string
	selectedName string
	containers   []messages.Container
	notification string
	pendingCtrlW bool
}

// NewDockerTab creates a new Docker tab.
func NewDockerTab(client *docker.Client, streamMgr *logpkg.StreamManager) *DockerTab {
	sidebar := components.NewSidebar()
	sidebar.SetFocused(true)

	return &DockerTab{
		client:       client,
		streamMgr:    streamMgr,
		sidebar:      sidebar,
		logView:      components.NewLogView(),
		detailPane:   components.NewDetailPane(),
		modal:        components.NewModal(),
		focusSidebar: true,
		rightPane:    paneLogs,
	}
}

func (t *DockerTab) Title() string { return "Docker" }

func (t *DockerTab) SetSize(width, height int) {
	t.width = width
	t.height = height

	sidebarWidth := width * 15 / 100
	if sidebarWidth < 20 {
		sidebarWidth = 20
	}
	rightWidth := width - sidebarWidth
	t.sidebar.SetSize(sidebarWidth, height)
	t.sidebar.SetYOffset(2) // tab bar height
	t.logView.SetSize(rightWidth, height)
	t.detailPane.SetSize(rightWidth, height)
	t.modal.SetSize(width, height)
}

func (t *DockerTab) Init() tea.Cmd {
	return tea.Batch(
		t.fetchContainers(),
		t.tickRefresh(),
	)
}

func (t *DockerTab) Update(msg tea.Msg) (ui.TabModel, tea.Cmd) {
	// Modal takes priority over everything.
	if t.modal.Visible() {
		cmd := t.modal.Update(msg)
		return t, cmd
	}

	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case messages.ContainerListMsg:
		if msg.Err == nil && msg.Source == "docker" {
			t.containers = msg.Containers
			t.sidebar.SetItems(msg.Containers)

			if t.selected == "" {
				if item, ok := t.sidebar.SelectedItem(); ok {
					return t, t.selectContainer(item.ID, item.Name)
				}
			}
		}
		return t, nil

	case messages.LogBatchMsg:
		if msg.SourceID == t.selected {
			t.logView.AppendLines(msg.Lines)
		}
		return t, t.waitForLogs(msg.SourceID)

	case messages.LogStreamErrorMsg:
		return t, nil

	case messages.ContainerActionMsg:
		if msg.Err == nil {
			t.setNotification(fmt.Sprintf("%s: %s OK", msg.Action, msg.Name))
			cmds = append(cmds, t.fetchContainers())
		} else {
			t.setNotification(fmt.Sprintf("%s: %s failed: %v", msg.Action, msg.Name, msg.Err))
		}
		return t, tea.Batch(cmds...)

	case messages.ContainerInspectMsg:
		if msg.Err == nil {
			t.detailPane.SetContent(fmt.Sprintf(" Inspect: %s ", msg.ID[:12]), msg.Data)
			t.rightPane = paneDetail
			if !t.focusSidebar {
				t.logView.SetFocused(false)
				t.detailPane.SetFocused(true)
			}
		} else {
			t.setNotification(fmt.Sprintf("inspect failed: %v", msg.Err))
		}
		return t, nil

	case messages.ExecFinishedMsg:
		if msg.Err != nil {
			t.setNotification(fmt.Sprintf("exec failed: %v", msg.Err))
		}
		return t, nil

	case messages.LogExportedMsg:
		if msg.Err == nil {
			t.setNotification(fmt.Sprintf("exported to %s", msg.Path))
		} else {
			t.setNotification(fmt.Sprintf("export failed: %v", msg.Err))
		}
		return t, nil

	case clearNotificationMsg:
		t.notification = ""
		return t, nil

	case refreshTickMsg:
		return t, tea.Batch(t.fetchContainers(), t.tickRefresh())

	case tea.MouseClickMsg:
		mouse := msg.Mouse()
		sidebarWidth := t.width * 15 / 100
		if sidebarWidth < 20 {
			sidebarWidth = 20
		}
		if mouse.X < sidebarWidth {
			// Click on sidebar — focus it and forward the click.
			t.focusSidebar = true
			t.sidebar.SetFocused(true)
			t.logView.SetFocused(false)
			t.detailPane.SetFocused(false)
			cmd := t.sidebar.Update(msg)
			// Auto-select container if clicked on an item.
			if item, ok := t.sidebar.SelectedItem(); ok && item.ID != t.selected {
				return t, tea.Batch(cmd, t.selectContainer(item.ID, item.Name))
			}
			return t, cmd
		}
		// Click on right pane — focus it.
		t.focusSidebar = false
		t.sidebar.SetFocused(false)
		if t.rightPane == paneDetail {
			t.detailPane.SetFocused(true)
		} else {
			t.logView.SetFocused(true)
		}
		return t, nil

	case tea.KeyPressMsg:
		// Ctrl+W W to toggle pane focus (vim-style).
		s := msg.String()
		if t.pendingCtrlW {
			t.pendingCtrlW = false
			if s == "w" || s == "W" || s == "ctrl+w" || s == "ctrl+W" { //nolint:goconst // key names
				t.toggleFocus()
				return t, nil
			}
		}
		if s == "ctrl+w" || s == "ctrl+W" { //nolint:goconst // key names
			t.pendingCtrlW = true
			return t, nil
		}

		// Global keys for this tab (regardless of focus).
		switch {
		case key.Matches(msg, theme.Keys.Describe):
			if t.selected != "" {
				if t.rightPane == paneDetail {
					// Toggle back to logs.
					t.rightPane = paneLogs
					t.detailPane.SetFocused(false)
					if !t.focusSidebar {
						t.logView.SetFocused(true)
					}
					return t, nil
				}
				return t, t.inspectContainer(t.selected)
			}
		}

		if t.focusSidebar {
			switch {
			case key.Matches(msg, theme.Keys.Enter):
				// Let sidebar handle group collapse first.
				cmd := t.sidebar.Update(msg)
				// If an item is selected (not a group), select it and move focus to logs.
				if item, ok := t.sidebar.SelectedItem(); ok {
					cmds := []tea.Cmd{cmd}
					if item.ID != t.selected {
						cmds = append(cmds, t.selectContainer(item.ID, item.Name))
					}
					t.toggleFocus() // move to log pane
					return t, tea.Batch(cmds...)
				}
				return t, cmd
			case key.Matches(msg, theme.Keys.Restart):
				if item, ok := t.sidebar.SelectedItem(); ok {
					// Restart is safe, no confirmation needed.
					t.setNotification(fmt.Sprintf("restarting %s...", item.Name))
					return t, t.restartContainer(item.ID, item.Name)
				}
			case key.Matches(msg, theme.Keys.Stop):
				if item, ok := t.sidebar.SelectedItem(); ok {
					id, name := item.ID, item.Name
					t.modal.Show(
						"Stop Container",
						fmt.Sprintf("Stop container %q?", name),
						func() tea.Cmd { return t.stopContainer(id, name) },
					)
					return t, nil
				}
			case key.Matches(msg, theme.Keys.Exec):
				if item, ok := t.sidebar.SelectedItem(); ok {
					return t, t.execShell(item.ID)
				}
			case key.Matches(msg, theme.Keys.Delete):
				if item, ok := t.sidebar.SelectedItem(); ok {
					id, name := item.ID, item.Name
					t.modal.Show(
						"Remove Container",
						fmt.Sprintf("Remove container %q? This cannot be undone.", name),
						func() tea.Cmd { return t.removeContainer(id, name) },
					)
					return t, nil
				}
			}

			prevItem, _ := t.sidebar.SelectedItem()
			cmd := t.sidebar.Update(msg)
			cmds = append(cmds, cmd)

			if item, ok := t.sidebar.SelectedItem(); ok && item.ID != prevItem.ID {
				cmds = append(cmds, t.selectContainer(item.ID, item.Name))
			}
		} else {
			if t.rightPane == paneDetail {
				cmd := t.detailPane.Update(msg)
				cmds = append(cmds, cmd)
			} else {
				cmd := t.logView.Update(msg)
				cmds = append(cmds, cmd)
			}
		}
	}

	return t, tea.Batch(cmds...)
}

func (t *DockerTab) View() string {
	sidebarWidth := t.width * 15 / 100
	if sidebarWidth < 20 {
		sidebarWidth = 20
	}

	var rightView string
	if t.rightPane == paneDetail {
		rightView = t.detailPane.View()
	} else {
		rightView = t.logView.View()
	}

	base := layout.HorizontalSplit(
		t.sidebar.View(),
		rightView,
		sidebarWidth,
		t.width,
		t.height,
	)

	// Overlay modal if visible.
	if t.modal.Visible() {
		return t.modal.View()
	}

	return base
}

// --- notifications ---

func (t *DockerTab) toggleFocus() {
	t.focusSidebar = !t.focusSidebar
	t.sidebar.SetFocused(t.focusSidebar)
	if t.focusSidebar {
		t.logView.SetFocused(false)
		t.detailPane.SetFocused(false)
	} else if t.rightPane == paneDetail {
		t.detailPane.SetFocused(true)
	} else {
		t.logView.SetFocused(true)
	}
}

func (t *DockerTab) setNotification(msg string) {
	t.notification = msg
}

// Notification returns the current notification text.
func (t *DockerTab) Notification() string {
	return t.notification
}

type clearNotificationMsg struct{}

// --- container actions ---

func (t *DockerTab) selectContainer(id, name string) tea.Cmd {
	if t.selected != "" {
		t.streamMgr.StopStream(t.selected)
	}

	t.selected = id
	t.selectedName = name
	t.logView.Clear()
	t.logView.SetSourceLabel(fmt.Sprintf(" %s ", name))

	// Switch back to logs pane when selecting a new container.
	t.rightPane = paneLogs
	t.detailPane.SetFocused(false)
	if !t.focusSidebar {
		t.logView.SetFocused(true)
	}

	return func() tea.Msg {
		ctx := context.Background()
		reader, err := t.client.GetLogs(ctx, id, 100)
		if err != nil {
			return messages.LogStreamErrorMsg{SourceID: id, Err: err}
		}

		ch := t.streamMgr.StartStream(id, reader, "docker")
		return readLogBatch(id, ch)
	}
}

func (t *DockerTab) fetchContainers() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		containers, err := t.client.ListContainers(ctx)
		return messages.ContainerListMsg{
			Containers: containers,
			Source:     "docker",
			Err:        err,
		}
	}
}

func (t *DockerTab) restartContainer(id, name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err := t.client.RestartContainer(ctx, id)
		return messages.ContainerActionMsg{Action: "restart", ID: id, Name: name, Err: err}
	}
}

func (t *DockerTab) stopContainer(id, name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		err := t.client.StopContainer(ctx, id)
		return messages.ContainerActionMsg{Action: "stop", ID: id, Name: name, Err: err}
	}
}

func (t *DockerTab) removeContainer(id, name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		err := t.client.RemoveContainer(ctx, id)
		return messages.ContainerActionMsg{Action: "remove", ID: id, Name: name, Err: err}
	}
}

func (t *DockerTab) inspectContainer(id string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		data, err := t.client.InspectContainer(ctx, id)
		return messages.ContainerInspectMsg{ID: id, Data: data, Err: err}
	}
}

func (t *DockerTab) waitForLogs(sourceID string) tea.Cmd {
	ch := t.streamMgr.GetChannel(sourceID)
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		return readLogBatch(sourceID, ch)
	}
}

func (t *DockerTab) execShell(containerID string) tea.Cmd {
	c := exec.CommandContext(context.Background(), "docker", "exec", "-it", containerID, "sh", "-c", "if command -v bash >/dev/null 2>&1; then bash; else sh; fi") //nolint:gosec // intentional exec into user-selected container
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return messages.ExecFinishedMsg{Err: err}
	})
}

func readLogBatch(sourceID string, ch <-chan messages.LogLine) tea.Msg {
	batch := make([]messages.LogLine, 0, 100)
	timeout := time.NewTimer(50 * time.Millisecond)
	defer timeout.Stop()

	line, ok := <-ch
	if !ok {
		return messages.LogStreamErrorMsg{SourceID: sourceID, Err: io.EOF}
	}
	batch = append(batch, line)

	for {
		select {
		case line, ok := <-ch:
			if !ok {
				return messages.LogBatchMsg{Lines: batch, SourceID: sourceID}
			}
			batch = append(batch, line)
			if len(batch) >= 100 {
				return messages.LogBatchMsg{Lines: batch, SourceID: sourceID}
			}
		case <-timeout.C:
			return messages.LogBatchMsg{Lines: batch, SourceID: sourceID}
		}
	}
}

type refreshTickMsg struct{}

func (t *DockerTab) tickRefresh() tea.Cmd {
	return tea.Tick(5*time.Second, func(time.Time) tea.Msg {
		return refreshTickMsg{}
	})
}
