package tabs

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	gitlabpkg "github.com/abhishek-rana/lazydev/internal/gitlab"
	"github.com/abhishek-rana/lazydev/internal/ui"
	"github.com/abhishek-rana/lazydev/internal/ui/components"
	"github.com/abhishek-rana/lazydev/internal/ui/theme"
	"github.com/abhishek-rana/lazydev/pkg/messages"
)

type pipelineRightPane int

const (
	pipelineRightDetail pipelineRightPane = iota
	pipelineRightJobLog
)

// PipelinesTab displays GitLab CI pipelines.
type PipelinesTab struct {
	client          *gitlabpkg.Client
	sidebar         components.Sidebar
	detailPane      components.DetailPane
	logView         components.LogView
	modal           components.Modal
	rightPane       pipelineRightPane
	focusSidebar    bool
	width           int
	height          int
	selectedID      int64
	pipelines       []messages.GitLabPipeline
	jobs            []messages.GitLabJob
	selectedJobIdx  int
	notification    string
	pendingCtrlW    bool
	refreshS        int
	fetchSeq        uint64
	pendingFetch    string
	needsAutoSelect bool
}

// NewPipelinesTab creates a new GitLab Pipelines tab.
func NewPipelinesTab(client *gitlabpkg.Client, refreshS int) *PipelinesTab {
	sidebar := components.NewSidebar()
	sidebar.SetFocused(true)
	if refreshS <= 0 {
		refreshS = 30
	}
	return &PipelinesTab{
		client:       client,
		sidebar:      sidebar,
		detailPane:   components.NewDetailPane(),
		logView:      components.NewLogView(),
		modal:        components.NewModal(),
		focusSidebar: true,
		refreshS:     refreshS,
	}
}

func (t *PipelinesTab) Title() string { return "Pipelines" }

func (t *PipelinesTab) SetSize(width, height int) {
	t.width = width
	t.height = height
	sidebarWidth := width * 25 / 100
	if sidebarWidth < 30 {
		sidebarWidth = 30
	}
	rightWidth := width - sidebarWidth
	t.sidebar.SetSize(sidebarWidth, height)
	t.sidebar.SetYOffset(2)
	t.detailPane.SetSize(rightWidth, height)
	t.logView.SetSize(rightWidth, height)
	t.logView.SetOffset(sidebarWidth, 2)
	t.modal.SetSize(width, height)
}

func (t *PipelinesTab) Init() tea.Cmd {
	return tea.Batch(t.fetchPipelines(), t.tickRefresh())
}

func (t *PipelinesTab) Update(msg tea.Msg) (ui.TabModel, tea.Cmd) {
	if t.modal.Visible() {
		cmd := t.modal.Update(msg)
		return t, cmd
	}

	switch msg := msg.(type) {
	case messages.PipelineListMsg:
		if msg.Err != nil {
			t.notification = fmt.Sprintf("pipelines: %v", msg.Err)
			return t, nil
		}
		t.pipelines = nil
		var containers []messages.Container
		for _, p := range msg.Pipelines {
			t.pipelines = append(t.pipelines, p)
			containers = append(containers, pipelineToContainer(p, "My pipelines"))
		}
		t.sidebar.SetItems(containers)
		// Flag auto-select for when the tab becomes active.
		if t.selectedID == 0 {
			t.needsAutoSelect = true
		}
		return t, nil

	case pipelineDetailFetchMsg:
		if msg.itemID == t.pendingFetch {
			t.detailPane.SetContent("Loading...", "Fetching pipeline jobs...")
			return t, t.selectPipeline(msg.itemID)
		}
		return t, nil

	case pipelineJobsResultMsg:
		if msg.seq != t.fetchSeq {
			return t, nil
		}
		if msg.err != nil {
			t.notification = fmt.Sprintf("jobs: %v", msg.err)
			return t, nil
		}
		t.jobs = msg.jobs
		t.selectedJobIdx = 0
		pipeline := t.findPipeline(msg.pipelineID)
		if pipeline != nil {
			detail := gitlabpkg.FormatPipelineDetail(*pipeline, msg.jobs)
			t.detailPane.SetContent(fmt.Sprintf("Pipeline #%d", pipeline.ID), detail)
		}
		t.rightPane = pipelineRightDetail
		return t, nil

	case messages.JobLogMsg:
		if msg.Err != nil {
			t.notification = fmt.Sprintf("job log: %v", msg.Err)
			return t, nil
		}
		t.logView.Clear()
		lines := convertJobLogToLines(msg.Log, msg.JobID)
		t.logView.AppendLines(lines)
		t.rightPane = pipelineRightJobLog
		return t, nil

	case messages.MRActionMsg:
		// Reused for pipeline actions.
		if msg.Err != nil {
			t.notification = fmt.Sprintf("%s failed: %v", msg.Action, msg.Err)
		} else {
			t.notification = msg.Action + " done"
		}
		return t, t.fetchPipelines()

	case messages.TabActivatedMsg:
		if t.needsAutoSelect {
			t.needsAutoSelect = false
			if item, ok := t.sidebar.SelectedItem(); ok {
				return t, t.selectPipeline(item.ID)
			}
		}
		return t, nil

	case pipelineRefreshTickMsg:
		return t, tea.Batch(t.fetchPipelines(), t.tickRefresh())

	case tea.MouseClickMsg:
		mouse := msg.Mouse()
		sidebarWidth := t.width * 25 / 100
		if sidebarWidth < 30 {
			sidebarWidth = 30
		}
		if mouse.X < sidebarWidth {
			t.focusSidebar = true
			t.sidebar.SetFocused(true)
			t.detailPane.SetFocused(false)
			t.logView.SetFocused(false)
			cmd := t.sidebar.Update(msg)
			if item, ok := t.sidebar.SelectedItem(); ok {
				return t, tea.Batch(cmd, t.selectPipeline(item.ID))
			}
			return t, cmd
		}
		t.focusSidebar = false
		t.sidebar.SetFocused(false)
		if t.rightPane == pipelineRightJobLog {
			t.logView.SetFocused(true)
			cmd := t.logView.Update(msg)
			return t, cmd
		}
		t.detailPane.SetFocused(true)
		cmd := t.detailPane.Update(msg)
		return t, cmd

	case tea.MouseWheelMsg:
		mouse := msg.Mouse()
		sidebarWidth := t.width * 25 / 100
		if sidebarWidth < 30 {
			sidebarWidth = 30
		}
		if mouse.X >= sidebarWidth {
			if t.rightPane == pipelineRightJobLog {
				cmd := t.logView.Update(msg)
				return t, cmd
			}
			cmd := t.detailPane.Update(msg)
			return t, cmd
		}
		return t, nil

	case tea.KeyPressMsg:
		s := msg.String()
		if t.pendingCtrlW {
			t.pendingCtrlW = false
			if s == "w" || s == "W" || s == "ctrl+w" || s == "ctrl+W" { //nolint:goconst // key names
				t.toggleFocus()
				return t, nil
			}
		}

		switch s {
		case "ctrl+w", "ctrl+W":
			t.pendingCtrlW = true
			return t, nil
		case "alt+w", "alt+W": //nolint:goconst // key names
			t.toggleFocus()
			return t, nil
		}

		if t.focusSidebar {
			return t.updateSidebar(msg)
		}

		// Right pane input.
		if t.rightPane == pipelineRightJobLog {
			if key.Matches(msg, theme.Keys.Back) {
				t.rightPane = pipelineRightDetail
				t.logView.SetFocused(false)
				t.detailPane.SetFocused(true)
				return t, nil
			}
			cmd := t.logView.Update(msg)
			return t, cmd
		}
		cmd := t.detailPane.Update(msg)
		return t, cmd
	}

	return t, nil
}

func (t *PipelinesTab) updateSidebar(msg tea.KeyPressMsg) (ui.TabModel, tea.Cmd) {
	switch {
	case key.Matches(msg, theme.Keys.Enter):
		if item, ok := t.sidebar.SelectedItem(); ok {
			t.focusSidebar = false
			t.sidebar.SetFocused(false)
			t.detailPane.SetFocused(true)
			t.detailPane.SetContent("Loading...", "Fetching pipeline jobs...")
			return t, t.selectPipeline(item.ID)
		}
	case msg.String() == "o":
		if p := t.findSelectedPipeline(); p != nil && p.WebURL != "" {
			_ = openBrowser(p.WebURL)
		}
	case msg.String() == "R":
		if p := t.findSelectedPipeline(); p != nil {
			pid := p.ID
			t.modal.Show("Retry Pipeline", fmt.Sprintf("Retry pipeline #%d?", p.ID), func() tea.Cmd {
				return t.retryPipeline(pid)
			})
			return t, nil
		}
	case msg.String() == "C":
		if p := t.findSelectedPipeline(); p != nil {
			pid := p.ID
			t.modal.Show("Cancel Pipeline", fmt.Sprintf("Cancel pipeline #%d?", p.ID), func() tea.Cmd {
				return t.cancelPipeline(pid)
			})
			return t, nil
		}
	default:
		prevItem, _ := t.sidebar.SelectedItem()
		cmd := t.sidebar.Update(msg)
		if newItem, ok := t.sidebar.SelectedItem(); ok && newItem.ID != prevItem.ID {
			t.pendingFetch = newItem.ID
			return t, tea.Batch(cmd, tea.Tick(150*time.Millisecond, func(time.Time) tea.Msg {
				return pipelineDetailFetchMsg{itemID: newItem.ID}
			}))
		}
		return t, cmd
	}
	return t, nil
}

func (t *PipelinesTab) View() string {
	left := t.sidebar.View()
	var right string
	if t.rightPane == pipelineRightJobLog {
		right = t.logView.View()
	} else {
		right = t.detailPane.View()
	}

	view := joinHorizontal(left, right, t.height)
	if t.modal.Visible() {
		return t.modal.View()
	}
	return view
}

func (t *PipelinesTab) Notification() string {
	n := t.notification
	t.notification = ""
	return n
}

func (t *PipelinesTab) toggleFocus() {
	t.focusSidebar = !t.focusSidebar
	t.sidebar.SetFocused(t.focusSidebar)
	if t.rightPane == pipelineRightJobLog {
		t.logView.SetFocused(!t.focusSidebar)
	} else {
		t.detailPane.SetFocused(!t.focusSidebar)
	}
}

func (t *PipelinesTab) fetchPipelines() tea.Cmd {
	return func() tea.Msg {
		pipelines, err := t.client.ListMyPipelines()
		return messages.PipelineListMsg{Pipelines: pipelines, Err: err}
	}
}

func (t *PipelinesTab) selectPipeline(id string) tea.Cmd {
	var pid int64
	fmt.Sscanf(id, "%d", &pid) //nolint:errcheck,gosec // best effort
	t.selectedID = pid
	t.fetchSeq++
	seq := t.fetchSeq

	return func() tea.Msg {
		jobs, err := t.client.GetPipelineJobs(pid)
		return pipelineJobsResultMsg{seq: seq, pipelineID: pid, jobs: jobs, err: err}
	}
}

func (t *PipelinesTab) retryPipeline(id int64) tea.Cmd {
	return func() tea.Msg {
		err := t.client.RetryPipeline(id)
		return messages.MRActionMsg{Action: "retry pipeline", Err: err}
	}
}

func (t *PipelinesTab) cancelPipeline(id int64) tea.Cmd {
	return func() tea.Msg {
		err := t.client.CancelPipeline(id)
		return messages.MRActionMsg{Action: "cancel pipeline", Err: err}
	}
}

type pipelineRefreshTickMsg struct{}

type pipelineDetailFetchMsg struct{ itemID string }

type pipelineJobsResultMsg struct {
	seq        uint64
	pipelineID int64
	jobs       []messages.GitLabJob
	err        error
}

func (t *PipelinesTab) tickRefresh() tea.Cmd {
	return tea.Tick(time.Duration(t.refreshS)*time.Second, func(time.Time) tea.Msg {
		return pipelineRefreshTickMsg{}
	})
}

func (t *PipelinesTab) findSelectedPipeline() *messages.GitLabPipeline {
	item, ok := t.sidebar.SelectedItem()
	if !ok {
		return nil
	}
	var id int64
	fmt.Sscanf(item.ID, "%d", &id) //nolint:errcheck,gosec // best effort
	return t.findPipeline(id)
}

func (t *PipelinesTab) findPipeline(id int64) *messages.GitLabPipeline {
	for i := range t.pipelines {
		if t.pipelines[i].ID == id {
			return &t.pipelines[i]
		}
	}
	return nil
}

func pipelineToContainer(p messages.GitLabPipeline, group string) messages.Container {
	state := messages.StatePending
	switch p.Status {
	case "success":
		state = messages.StateRunning
	case "failed":
		state = messages.StateError
	case "running":
		state = messages.StateRestarting
	case "canceled":
		state = messages.StateStopped
	}
	icon := gitlabpkg.PipelineStatusIcon(p.Status)
	var name string
	if p.MRIid != "" {
		name = fmt.Sprintf("#%d !%s [%s] %s", p.ID, p.MRIid, p.PipelineType, icon)
	} else {
		name = fmt.Sprintf("#%d %s %s", p.ID, p.Ref, icon)
	}
	age := relativeTime(p.CreatedAt)
	if age != "" {
		name += " " + age
	}
	return messages.Container{
		ID:    fmt.Sprintf("%d", p.ID),
		Name:  name,
		State: state,
		Group: group,
	}
}

func convertJobLogToLines(log string, jobID int64) []messages.LogLine {
	rawLines := strings.Split(log, "\n")
	result := make([]messages.LogLine, 0, len(rawLines))
	for _, line := range rawLines {
		if line == "" {
			continue
		}
		result = append(result, messages.LogLine{
			Source:   "gitlab-job",
			SourceID: fmt.Sprintf("job-%d", jobID),
			Text:     line,
			Time:     time.Now(),
		})
	}
	return result
}
