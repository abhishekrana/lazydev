package tabs

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/abhishek-rana/lazydk/internal/kube"
	logpkg "github.com/abhishek-rana/lazydk/internal/log"
	"github.com/abhishek-rana/lazydk/internal/ui"
	"github.com/abhishek-rana/lazydk/internal/ui/components"
	"github.com/abhishek-rana/lazydk/internal/ui/layout"
	"github.com/abhishek-rana/lazydk/internal/ui/theme"
	"github.com/abhishek-rana/lazydk/pkg/messages"
)

// kubeRightPane tracks which pane is shown on the right.
type kubeRightPane int

const (
	kubeRightLogs kubeRightPane = iota
	kubeRightDetail
)

// KubeTab is the Kubernetes pod/resource management tab.
type KubeTab struct {
	client       *kube.Client
	streamMgr    *logpkg.StreamManager
	sidebar      components.Sidebar
	logView      components.LogView
	detailPane   components.DetailPane
	modal        components.Modal
	inputModal   components.InputModal
	focusSidebar bool
	rightPane    kubeRightPane
	width        int
	height       int
	selected     string // pod name
	selectedNs   string // namespace of selected pod
	selectedName string
	containers   []messages.Container
	notification string
	pendingCtrlW bool
}

// NewKubeTab creates a new Kubernetes tab.
func NewKubeTab(client *kube.Client, streamMgr *logpkg.StreamManager) *KubeTab {
	sidebar := components.NewSidebar()
	sidebar.SetFocused(true)

	return &KubeTab{
		client:       client,
		streamMgr:    streamMgr,
		sidebar:      sidebar,
		inputModal:   components.NewInputModal(),
		logView:      components.NewLogView(),
		detailPane:   components.NewDetailPane(),
		modal:        components.NewModal(),
		focusSidebar: true,
		rightPane:    kubeRightLogs,
	}
}

func (t *KubeTab) Title() string { return "Kubernetes" }

func (t *KubeTab) SetSize(width, height int) {
	t.width = width
	t.height = height

	sidebarWidth := width * 30 / 100
	if sidebarWidth < 20 {
		sidebarWidth = 20
	}
	rightWidth := width - sidebarWidth
	t.sidebar.SetSize(sidebarWidth, height)
	t.sidebar.SetYOffset(2) // tab bar height
	t.logView.SetSize(rightWidth, height)
	t.detailPane.SetSize(rightWidth, height)
	t.modal.SetSize(width, height)
	t.inputModal.SetSize(width, height)
}

func (t *KubeTab) Init() tea.Cmd {
	return tea.Batch(
		t.fetchPods(),
		t.tickRefresh(),
	)
}

func (t *KubeTab) Update(msg tea.Msg) (ui.TabModel, tea.Cmd) {
	// Input modal takes priority.
	if t.inputModal.Visible() {
		cmd := t.inputModal.Update(msg)
		return t, cmd
	}

	// Modal takes priority.
	if t.modal.Visible() {
		cmd := t.modal.Update(msg)
		return t, cmd
	}

	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case messages.ContainerListMsg:
		if msg.Err == nil && msg.Source == "kubernetes" {
			t.containers = msg.Containers
			t.sidebar.SetItems(msg.Containers)

			if t.selected == "" {
				if item, ok := t.sidebar.SelectedItem(); ok {
					return t, t.selectPod(item.ID, item.Name, item.Group)
				}
			}
		}
		return t, nil

	case messages.LogBatchMsg:
		if msg.SourceID == t.podStreamID() {
			t.logView.AppendLines(msg.Lines)
		}
		return t, t.waitForLogs(msg.SourceID)

	case messages.LogStreamErrorMsg:
		return t, nil

	case messages.ContainerActionMsg:
		if msg.Err == nil {
			t.setNotification(fmt.Sprintf("%s: %s OK", msg.Action, msg.Name))
			cmds = append(cmds, t.fetchPods())
		} else {
			t.setNotification(fmt.Sprintf("%s: %s failed: %v", msg.Action, msg.Name, msg.Err))
		}
		return t, tea.Batch(cmds...)

	case messages.ContainerInspectMsg:
		if msg.Err == nil {
			t.detailPane.SetContent(fmt.Sprintf(" %s ", msg.ID), msg.Data)
			t.rightPane = kubeRightDetail
			if !t.focusSidebar {
				t.logView.SetFocused(false)
				t.detailPane.SetFocused(true)
			}
		} else {
			t.setNotification(fmt.Sprintf("describe failed: %v", msg.Err))
		}
		return t, nil

	case messages.ExecFinishedMsg:
		if msg.Err != nil {
			t.setNotification(fmt.Sprintf("exec failed: %v", msg.Err))
		}
		return t, nil

	case messages.ScaleMsg:
		if msg.Err == nil {
			t.setNotification(fmt.Sprintf("scaled %s to %d replicas", msg.Name, msg.Replicas))
		} else {
			t.setNotification(fmt.Sprintf("scale failed: %v", msg.Err))
		}
		return t, t.fetchPods()

	case kubeRefreshTickMsg:
		return t, tea.Batch(t.fetchPods(), t.tickRefresh())

	case tea.MouseClickMsg:
		mouse := msg.Mouse()
		sidebarWidth := t.width * 30 / 100
		if sidebarWidth < 20 {
			sidebarWidth = 20
		}
		if mouse.X < sidebarWidth {
			t.focusSidebar = true
			t.sidebar.SetFocused(true)
			t.logView.SetFocused(false)
			t.detailPane.SetFocused(false)
			cmd := t.sidebar.Update(msg)
			if item, ok := t.sidebar.SelectedItem(); ok && (item.ID != t.selected || item.Group != t.selectedNs) {
				return t, tea.Batch(cmd, t.selectPod(item.ID, item.Name, item.Group))
			}
			return t, cmd
		}
		t.focusSidebar = false
		t.sidebar.SetFocused(false)
		if t.rightPane == kubeRightDetail {
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
			if s == "w" || s == "W" || s == "ctrl+w" || s == "ctrl+W" {
				t.toggleFocus()
				return t, nil
			}
		}
		if s == "ctrl+w" || s == "ctrl+W" {
			t.pendingCtrlW = true
			return t, nil
		}

		// Global keys for this tab.
		switch {
		case key.Matches(msg, theme.Keys.Describe):
			if t.selected != "" {
				if t.rightPane == kubeRightDetail {
					t.rightPane = kubeRightLogs
					t.detailPane.SetFocused(false)
					if !t.focusSidebar {
						t.logView.SetFocused(true)
					}
					return t, nil
				}
				return t, t.describePod(t.selectedNs, t.selected)
			}
		case key.Matches(msg, theme.Keys.Yaml):
			if t.selected != "" {
				return t, t.getPodYAML(t.selectedNs, t.selected)
			}
		}

		if t.focusSidebar {
			switch {
			case key.Matches(msg, theme.Keys.Enter):
				cmd := t.sidebar.Update(msg)
				if item, ok := t.sidebar.SelectedItem(); ok {
					if item.ID != t.selected || item.Group != t.selectedNs {
						return t, tea.Batch(cmd, t.selectPod(item.ID, item.Name, item.Group))
					}
				}
				return t, cmd
			case key.Matches(msg, theme.Keys.Delete):
				if item, ok := t.sidebar.SelectedItem(); ok {
					ns, name := item.Group, item.ID
					t.modal.Show(
						"Delete Pod",
						fmt.Sprintf("Delete pod %q in namespace %q?", name, ns),
						func() tea.Cmd { return t.deletePod(ns, name) },
					)
					return t, nil
				}
			case key.Matches(msg, theme.Keys.Restart):
				if item, ok := t.sidebar.SelectedItem(); ok {
					ns, name := item.Group, item.ID
					t.setNotification(fmt.Sprintf("deleting pod %s (will be recreated)...", name))
					return t, t.deletePod(ns, name)
				}
			case key.Matches(msg, theme.Keys.Exec):
				if item, ok := t.sidebar.SelectedItem(); ok {
					return t, t.execShell(item.Group, item.ID)
				}
			case key.Matches(msg, theme.Keys.PortFwd):
				if item, ok := t.sidebar.SelectedItem(); ok {
					ns, name := item.Group, item.ID
					t.inputModal.Show(
						fmt.Sprintf("Port-forward %s", name),
						"local:remote (e.g. 8080:80)",
						func(value string) tea.Cmd {
							return t.portForward(ns, name, value)
						},
					)
					return t, nil
				}
			case key.Matches(msg, theme.Keys.Scale):
				if item, ok := t.sidebar.SelectedItem(); ok {
					ns, name := item.Group, item.ID
					t.inputModal.Show(
						fmt.Sprintf("Scale deployment %s", name),
						"replicas (e.g. 3)",
						func(value string) tea.Cmd {
							return t.scaleDeployment(ns, name, value)
						},
					)
					return t, nil
				}
			}

			prevItem, _ := t.sidebar.SelectedItem()
			cmd := t.sidebar.Update(msg)
			cmds = append(cmds, cmd)

			if item, ok := t.sidebar.SelectedItem(); ok && (item.ID != prevItem.ID || item.Group != prevItem.Group) {
				cmds = append(cmds, t.selectPod(item.ID, item.Name, item.Group))
			}
		} else {
			if t.rightPane == kubeRightDetail {
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

func (t *KubeTab) View() string {
	sidebarWidth := t.width * 30 / 100
	if sidebarWidth < 20 {
		sidebarWidth = 20
	}

	var rightView string
	if t.rightPane == kubeRightDetail {
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

	if t.inputModal.Visible() {
		return t.inputModal.View()
	}

	if t.modal.Visible() {
		return t.modal.View()
	}

	return base
}

// Notification implements the Notifier interface.
func (t *KubeTab) Notification() string {
	return t.notification
}

func (t *KubeTab) toggleFocus() {
	t.focusSidebar = !t.focusSidebar
	t.sidebar.SetFocused(t.focusSidebar)
	if t.focusSidebar {
		t.logView.SetFocused(false)
		t.detailPane.SetFocused(false)
	} else if t.rightPane == kubeRightDetail {
		t.detailPane.SetFocused(true)
	} else {
		t.logView.SetFocused(true)
	}
}

func (t *KubeTab) setNotification(msg string) {
	t.notification = msg
}

func (t *KubeTab) podStreamID() string {
	return t.selectedNs + "/" + t.selected
}

// --- pod actions ---

func (t *KubeTab) selectPod(id, name, namespace string) tea.Cmd {
	oldStreamID := t.podStreamID()
	if t.selected != "" {
		t.streamMgr.StopStream(oldStreamID)
	}

	t.selected = id
	t.selectedName = name
	t.selectedNs = namespace
	t.logView.Clear()
	t.logView.SetSourceLabel(fmt.Sprintf(" %s/%s ", namespace, name))

	t.rightPane = kubeRightLogs
	t.detailPane.SetFocused(false)
	if !t.focusSidebar {
		t.logView.SetFocused(true)
	}

	streamID := namespace + "/" + id
	return func() tea.Msg {
		ctx := context.Background()
		reader, err := t.client.GetPodLogs(ctx, namespace, id, "", 100)
		if err != nil {
			return messages.LogStreamErrorMsg{SourceID: streamID, Err: err}
		}

		ch := t.streamMgr.StartStream(streamID, reader, "kubernetes")
		return readLogBatch(streamID, ch)
	}
}

func (t *KubeTab) fetchPods() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		pods, err := t.client.ListPods(ctx, "")
		return messages.ContainerListMsg{
			Containers: pods,
			Source:     "kubernetes",
			Err:        err,
		}
	}
}

func (t *KubeTab) deletePod(namespace, name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		err := t.client.DeletePod(ctx, namespace, name)
		return messages.ContainerActionMsg{Action: "delete", ID: name, Name: name, Err: err}
	}
}

func (t *KubeTab) describePod(namespace, name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		data, err := t.client.DescribePod(ctx, namespace, name)
		return messages.ContainerInspectMsg{ID: namespace + "/" + name, Data: data, Err: err}
	}
}

func (t *KubeTab) getPodYAML(namespace, name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		data, err := t.client.GetPodYAML(ctx, namespace, name)
		return messages.ContainerInspectMsg{ID: namespace + "/" + name + " (YAML)", Data: data, Err: err}
	}
}

func (t *KubeTab) waitForLogs(sourceID string) tea.Cmd {
	ch := t.streamMgr.GetChannel(sourceID)
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		return readLogBatch(sourceID, ch)
	}
}

func (t *KubeTab) execShell(namespace, podName string) tea.Cmd {
	c := exec.CommandContext(context.Background(), "kubectl", "exec", "-it", "-n", namespace, podName, "--", "sh", "-c", "if command -v bash >/dev/null 2>&1; then bash; else sh; fi") //nolint:gosec // intentional exec into user-selected pod
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return messages.ExecFinishedMsg{Err: err}
	})
}

func (t *KubeTab) portForward(namespace, podName, ports string) tea.Cmd {
	// Run kubectl port-forward in the background via exec.
	c := exec.CommandContext(context.Background(), "kubectl", "port-forward", "-n", namespace, podName, ports) //nolint:gosec // intentional port-forward to user-selected pod
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return messages.ExecFinishedMsg{Err: err}
	})
}

func (t *KubeTab) scaleDeployment(namespace, name, replicaStr string) tea.Cmd {
	replicas, err := strconv.Atoi(replicaStr)
	if err != nil || replicas < 0 || replicas > 1000 {
		return func() tea.Msg {
			return messages.ScaleMsg{Name: name, Err: fmt.Errorf("invalid replica count: %s", replicaStr)}
		}
	}
	r := int32(replicas) //nolint:gosec // validated above
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err := t.client.ScaleDeployment(ctx, namespace, name, r)
		return messages.ScaleMsg{Name: name, Replicas: replicas, Err: err}
	}
}

type kubeRefreshTickMsg struct{}

func (t *KubeTab) tickRefresh() tea.Cmd {
	return tea.Tick(5*time.Second, func(time.Time) tea.Msg {
		return kubeRefreshTickMsg{}
	})
}
