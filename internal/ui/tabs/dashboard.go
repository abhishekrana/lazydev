package tabs

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/abhishek-rana/lazydev/internal/docker"
	"github.com/abhishek-rana/lazydev/internal/kube"
	"github.com/abhishek-rana/lazydev/internal/ui"
	"github.com/abhishek-rana/lazydev/internal/ui/components"
	"github.com/abhishek-rana/lazydev/pkg/messages"
)

// DashboardTab shows a table overview of all resources with status and metrics.
type DashboardTab struct {
	dockerClient *docker.Client
	kubeClient   *kube.Client
	table        components.Table
	width        int
	height       int
	rows         []messages.DashboardRow
	stats        map[string]messages.ResourceStats // keyed by source/name
}

// NewDashboardTab creates a new dashboard tab.
func NewDashboardTab(dockerClient *docker.Client, kubeClient *kube.Client) *DashboardTab {
	return &DashboardTab{
		dockerClient: dockerClient,
		kubeClient:   kubeClient,
		table:        components.NewTable(),
		stats:        make(map[string]messages.ResourceStats),
	}
}

func (t *DashboardTab) Title() string { return "Dashboard" }

func (t *DashboardTab) SetSize(width, height int) {
	t.width = width
	t.height = height
	t.table.SetSize(width, height)
}

func (t *DashboardTab) Init() tea.Cmd {
	cmds := []tea.Cmd{t.tickRefresh()}
	if t.dockerClient != nil {
		cmds = append(cmds, t.fetchDockerContainers(), t.fetchDockerStats())
	}
	if t.kubeClient != nil {
		cmds = append(cmds, t.fetchKubePods())
	}
	return tea.Batch(cmds...)
}

func (t *DashboardTab) Update(msg tea.Msg) (ui.TabModel, tea.Cmd) {
	switch msg := msg.(type) {
	case messages.ContainerListMsg:
		if msg.Err == nil {
			t.mergeContainers(msg.Containers, msg.Source)
			t.rebuildTable()
		}
		return t, nil

	case messages.ResourceStatsMsg:
		if msg.Err == nil {
			for _, s := range msg.Stats {
				key := s.Source + "/" + s.Name
				t.stats[key] = s
			}
			t.rebuildTable()
		}
		return t, nil

	case dashboardTickMsg:
		cmds := []tea.Cmd{t.tickRefresh()}
		if t.dockerClient != nil {
			cmds = append(cmds, t.fetchDockerContainers(), t.fetchDockerStats())
		}
		if t.kubeClient != nil {
			cmds = append(cmds, t.fetchKubePods())
		}
		return t, tea.Batch(cmds...)

	case tea.KeyPressMsg:
		cmd := t.table.Update(msg)
		return t, cmd
	}

	return t, nil
}

func (t *DashboardTab) View() string {
	return t.table.View()
}

// Notification implements the Notifier interface.
func (t *DashboardTab) Notification() string {
	return ""
}

// mergeContainers updates the rows for a given source.
func (t *DashboardTab) mergeContainers(containers []messages.Container, source string) {
	// Remove old rows from this source.
	var kept []messages.DashboardRow
	for _, r := range t.rows {
		if r.Source != source {
			kept = append(kept, r)
		}
	}

	// Add new rows.
	for _, c := range containers {
		rType := "container"
		if c.Source == "kubernetes" {
			rType = "pod"
		}
		kept = append(kept, messages.DashboardRow{
			Name:     c.Name,
			Type:     rType,
			Source:   c.Source,
			Group:    c.Group,
			Status:   c.Status,
			State:    c.State,
			Restarts: c.Restarts,
		})
	}

	t.rows = kept
}

func (t *DashboardTab) rebuildTable() {
	// Merge stats into rows.
	for i := range t.rows {
		key := t.rows[i].Source + "/" + t.rows[i].Name
		if s, ok := t.stats[key]; ok {
			t.rows[i].CPU = s.CPU
			t.rows[i].Memory = s.Memory
		}
	}
	t.table.SetRows(t.rows)
}

// --- data fetching ---

func (t *DashboardTab) fetchDockerContainers() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		containers, err := t.dockerClient.ListContainers(ctx)
		return messages.ContainerListMsg{Containers: containers, Source: "docker", Err: err}
	}
}

func (t *DashboardTab) fetchDockerStats() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		stats, err := t.dockerClient.ContainerStats(ctx)
		return messages.ResourceStatsMsg{Stats: stats, Source: "docker", Err: err}
	}
}

func (t *DashboardTab) fetchKubePods() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		pods, err := t.kubeClient.ListPods(ctx, "")
		return messages.ContainerListMsg{Containers: pods, Source: "kubernetes", Err: err}
	}
}

type dashboardTickMsg struct{}

func (t *DashboardTab) tickRefresh() tea.Cmd {
	return tea.Tick(5*time.Second, func(time.Time) tea.Msg {
		return dashboardTickMsg{}
	})
}
