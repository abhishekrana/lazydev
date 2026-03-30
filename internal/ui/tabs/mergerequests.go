package tabs

import (
	"fmt"
	"os"
	"os/exec"
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

// MRsTab displays GitLab merge requests.
type MRsTab struct {
	client       *gitlabpkg.Client
	sidebar      components.Sidebar
	detailPane   components.DetailPane
	modal        components.Modal
	focusSidebar bool
	width        int
	height       int
	selectedIID  int64
	mrs          []messages.GitLabMR
	notification string
	pendingCtrlW bool
	refreshS     int
	fetchSeq     uint64
	pendingFetch string
}

// NewMRsTab creates a new GitLab Merge Requests tab.
func NewMRsTab(client *gitlabpkg.Client, refreshS int) *MRsTab {
	sidebar := components.NewSidebar()
	sidebar.SetFocused(true)
	if refreshS <= 0 {
		refreshS = 30
	}
	return &MRsTab{
		client:       client,
		sidebar:      sidebar,
		detailPane:   components.NewDetailPane(),
		modal:        components.NewModal(),
		focusSidebar: true,
		refreshS:     refreshS,
	}
}

func (t *MRsTab) Title() string { return "MRs" }

func (t *MRsTab) SetSize(width, height int) {
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
	t.modal.SetSize(width, height)
}

func (t *MRsTab) Init() tea.Cmd {
	return tea.Batch(t.fetchMRs(), t.tickRefresh())
}

func (t *MRsTab) Update(msg tea.Msg) (ui.TabModel, tea.Cmd) {
	if t.modal.Visible() {
		cmd := t.modal.Update(msg)
		return t, cmd
	}

	switch msg := msg.(type) {
	case messages.MRListMsg:
		if msg.Err != nil {
			t.notification = fmt.Sprintf("MRs: %v", msg.Err)
			return t, nil
		}
		t.mrs = nil
		var containers []messages.Container
		for _, mr := range msg.Mine {
			t.mrs = append(t.mrs, mr)
			containers = append(containers, mrToContainer(mr, "My MRs"))
		}
		for _, mr := range msg.ReviewRequested {
			if !mrInList(mr, msg.Mine) {
				t.mrs = append(t.mrs, mr)
				containers = append(containers, mrToContainer(mr, "Review requested"))
			}
		}
		for _, mr := range msg.AllOpen {
			if !mrInList(mr, msg.Mine) && !mrInList(mr, msg.ReviewRequested) {
				t.mrs = append(t.mrs, mr)
				containers = append(containers, mrToContainer(mr, "All open"))
			}
		}
		t.sidebar.SetItems(containers)
		// Auto-select first item on initial load.
		if t.selectedIID == 0 {
			if item, ok := t.sidebar.SelectedItem(); ok {
				return t, t.selectMR(item.ID)
			}
		}
		return t, nil

	case mrDetailFetchMsg:
		if msg.itemID == t.pendingFetch {
			t.detailPane.SetContent("Loading...", "Fetching MR details...")
			return t, t.selectMR(msg.itemID)
		}
		return t, nil

	case mrDetailResultMsg:
		if msg.seq != t.fetchSeq {
			return t, nil
		}
		if msg.err != nil {
			t.notification = fmt.Sprintf("MR detail: %v", msg.err)
			return t, nil
		}
		detail := gitlabpkg.FormatMRDetail(msg.mr, msg.notes, t.detailPane.Width())
		t.detailPane.SetContent(fmt.Sprintf("!%d %s", msg.mr.IID, msg.mr.Title), detail)
		return t, nil

	case messages.MRActionMsg:
		if msg.Err != nil {
			t.notification = fmt.Sprintf("%s failed: %v", msg.Action, msg.Err)
		} else {
			t.notification = msg.Action + " done"
		}
		return t, t.fetchMRs()

	case messages.ExecFinishedMsg:
		return t, nil

	case mrRefreshTickMsg:
		return t, tea.Batch(t.fetchMRs(), t.tickRefresh())

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
			cmd := t.sidebar.Update(msg)
			if item, ok := t.sidebar.SelectedItem(); ok {
				return t, tea.Batch(cmd, t.selectMR(item.ID))
			}
			return t, cmd
		}
		t.focusSidebar = false
		t.sidebar.SetFocused(false)
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
		cmd := t.detailPane.Update(msg)
		return t, cmd
	}

	return t, nil
}

func (t *MRsTab) updateSidebar(msg tea.KeyPressMsg) (ui.TabModel, tea.Cmd) {
	switch {
	case key.Matches(msg, theme.Keys.Enter):
		if item, ok := t.sidebar.SelectedItem(); ok {
			t.focusSidebar = false
			t.sidebar.SetFocused(false)
			t.detailPane.SetFocused(true)
			t.detailPane.SetContent("Loading...", "Fetching MR details...")
			return t, t.selectMR(item.ID)
		}
	case msg.String() == "o":
		if mr := t.findSelectedMR(); mr != nil && mr.WebURL != "" {
			_ = openBrowser(mr.WebURL)
		}
	case msg.String() == "r":
		// Review in neovim with DiffviewOpen.
		if mr := t.findSelectedMR(); mr != nil {
			return t, t.reviewInNeovim(mr)
		}
	case msg.String() == "m":
		// Merge with confirmation.
		if mr := t.findSelectedMR(); mr != nil {
			t.modal.Show(
				"Merge MR",
				fmt.Sprintf("Merge !%d %s?", mr.IID, mr.Title),
				func() tea.Cmd {
					return t.mergeMR(mr.IID)
				},
			)
			return t, nil
		}
	case msg.String() == "A":
		if mr := t.findSelectedMR(); mr != nil {
			iid := mr.IID
			t.modal.Show("Approve MR", fmt.Sprintf("Approve !%d %s?", mr.IID, mr.Title), func() tea.Cmd {
				return t.approveMR(iid)
			})
			return t, nil
		}
	case msg.String() == "s":
		if mr := t.findSelectedMR(); mr != nil {
			if mr.State == "opened" {
				iid := mr.IID
				t.modal.Show("Close MR", fmt.Sprintf("Close !%d %s?", mr.IID, mr.Title), func() tea.Cmd {
					return t.closeMR(iid)
				})
			} else {
				iid := mr.IID
				t.modal.Show("Reopen MR", fmt.Sprintf("Reopen !%d %s?", mr.IID, mr.Title), func() tea.Cmd {
					return t.reopenMR(iid)
				})
			}
			return t, nil
		}
	case msg.String() == "c":
		if mr := t.findSelectedMR(); mr != nil {
			return t, t.commentOnMR(mr.IID)
		}
	default:
		prevItem, _ := t.sidebar.SelectedItem()
		cmd := t.sidebar.Update(msg)
		if newItem, ok := t.sidebar.SelectedItem(); ok && newItem.ID != prevItem.ID {
			t.pendingFetch = newItem.ID
			return t, tea.Batch(cmd, tea.Tick(150*time.Millisecond, func(time.Time) tea.Msg {
				return mrDetailFetchMsg{itemID: newItem.ID}
			}))
		}
		return t, cmd
	}
	return t, nil
}

func (t *MRsTab) View() string {
	left := t.sidebar.View()
	right := t.detailPane.View()

	view := joinHorizontal(left, right, t.height)
	if t.modal.Visible() {
		return t.modal.View()
	}
	return view
}

func (t *MRsTab) Notification() string {
	n := t.notification
	t.notification = ""
	return n
}

func (t *MRsTab) toggleFocus() {
	t.focusSidebar = !t.focusSidebar
	t.sidebar.SetFocused(t.focusSidebar)
	t.detailPane.SetFocused(!t.focusSidebar)
}

func (t *MRsTab) fetchMRs() tea.Cmd {
	return func() tea.Msg {
		mine, review, allOpen, err := t.client.ListMyMRs()
		return messages.MRListMsg{
			Mine:            mine,
			ReviewRequested: review,
			AllOpen:         allOpen,
			Err:             err,
		}
	}
}

func (t *MRsTab) selectMR(id string) tea.Cmd {
	var iid int64
	fmt.Sscanf(id, "%d", &iid) //nolint:errcheck,gosec // best effort
	t.selectedIID = iid
	t.fetchSeq++
	seq := t.fetchSeq

	return func() tea.Msg {
		mr, notes, err := t.client.GetMR(iid)
		return mrDetailResultMsg{seq: seq, mr: mr, notes: notes, err: err}
	}
}

func (t *MRsTab) approveMR(iid int64) tea.Cmd {
	return func() tea.Msg {
		err := t.client.ApproveMR(iid)
		return messages.MRActionMsg{Action: "approve", Err: err}
	}
}

func (t *MRsTab) mergeMR(iid int64) tea.Cmd {
	return func() tea.Msg {
		err := t.client.MergeMR(iid)
		return messages.MRActionMsg{Action: "merge", Err: err}
	}
}

func (t *MRsTab) closeMR(iid int64) tea.Cmd {
	return func() tea.Msg {
		err := t.client.CloseMR(iid)
		return messages.MRActionMsg{Action: "close MR", Err: err}
	}
}

func (t *MRsTab) reopenMR(iid int64) tea.Cmd {
	return func() tea.Msg {
		err := t.client.ReopenMR(iid)
		return messages.MRActionMsg{Action: "reopen MR", Err: err}
	}
}

func (t *MRsTab) commentOnMR(iid int64) tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}
	tmpFile := fmt.Sprintf("/tmp/lazydev-mr-comment-%d.md", iid)
	_ = os.WriteFile(tmpFile, []byte(""), 0o600) //nolint:gosec // temp file

	c := exec.Command(editor, tmpFile) //nolint:gosec,noctx // intentional editor open
	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			return messages.MRActionMsg{Action: "comment", Err: err}
		}
		body, readErr := os.ReadFile(tmpFile) //nolint:gosec // temp file we just created
		if readErr != nil || len(strings.TrimSpace(string(body))) == 0 {
			return messages.MRActionMsg{Action: "comment", Err: fmt.Errorf("empty comment")}
		}
		postErr := t.client.CommentOnMR(iid, string(body))
		_ = os.Remove(tmpFile)
		return messages.MRActionMsg{Action: "comment", Err: postErr}
	})
}

func (t *MRsTab) reviewInNeovim(mr *messages.GitLabMR) tea.Cmd {
	// Fetch the branch and open neovim with DiffviewOpen.
	fetchCmd := fmt.Sprintf("git fetch origin %s && git checkout %s", mr.SourceBranch, mr.SourceBranch)
	diffCmd := fmt.Sprintf("DiffviewOpen origin/%s...%s", mr.TargetBranch, mr.SourceBranch)

	shell := exec.Command("bash", "-c", //nolint:gosec,noctx // intentional shell exec
		fmt.Sprintf(`%s && nvim -c "%s"`, fetchCmd, diffCmd))
	return tea.ExecProcess(shell, func(err error) tea.Msg {
		return messages.ExecFinishedMsg{Err: err}
	})
}

type mrRefreshTickMsg struct{}

type mrDetailFetchMsg struct{ itemID string }

type mrDetailResultMsg struct {
	seq   uint64
	mr    messages.GitLabMR
	notes []messages.GitLabNote
	err   error
}

func (t *MRsTab) tickRefresh() tea.Cmd {
	return tea.Tick(time.Duration(t.refreshS)*time.Second, func(time.Time) tea.Msg {
		return mrRefreshTickMsg{}
	})
}

func (t *MRsTab) findSelectedMR() *messages.GitLabMR {
	item, ok := t.sidebar.SelectedItem()
	if !ok {
		return nil
	}
	var iid int64
	fmt.Sscanf(item.ID, "%d", &iid) //nolint:errcheck,gosec // best effort
	for i := range t.mrs {
		if t.mrs[i].IID == iid {
			return &t.mrs[i]
		}
	}
	return nil
}

func mrToContainer(mr messages.GitLabMR, group string) messages.Container {
	state := messages.StateRunning
	switch mr.State {
	case "merged":
		state = messages.StateStopped
	case "closed":
		state = messages.StateError
	}
	pipeline := ""
	switch mr.PipelineStatus {
	case "success":
		pipeline = " ✓"
	case "failed":
		pipeline = " ✗"
	case "running":
		pipeline = " ◌"
	}
	name := fmt.Sprintf("!%d %s%s", mr.IID, truncate(mr.Title, 35), pipeline)
	age := relativeTime(mr.UpdatedAt)
	if age != "" {
		name += " " + age
	}
	return messages.Container{
		ID:    fmt.Sprintf("%d", mr.IID),
		Name:  name,
		State: state,
		Group: group,
	}
}

func mrInList(mr messages.GitLabMR, list []messages.GitLabMR) bool {
	for _, m := range list {
		if m.IID == mr.IID {
			return true
		}
	}
	return false
}
