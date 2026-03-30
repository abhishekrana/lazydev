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

// IssuesTab displays GitLab issues.
type IssuesTab struct {
	client       *gitlabpkg.Client
	sidebar      components.Sidebar
	detailPane   components.DetailPane
	modal        components.Modal
	inputModal   components.InputModal
	focusSidebar bool
	width        int
	height       int
	selectedIID  int64
	issues       []messages.GitLabIssue // flat list for lookup
	notification string
	pendingCtrlW bool
	refreshS     int
}

// NewIssuesTab creates a new GitLab Issues tab.
func NewIssuesTab(client *gitlabpkg.Client, refreshS int) *IssuesTab {
	sidebar := components.NewSidebar()
	sidebar.SetFocused(true)
	if refreshS <= 0 {
		refreshS = 30
	}
	return &IssuesTab{
		client:       client,
		sidebar:      sidebar,
		detailPane:   components.NewDetailPane(),
		modal:        components.NewModal(),
		inputModal:   components.NewInputModal(),
		focusSidebar: true,
		refreshS:     refreshS,
	}
}

func (t *IssuesTab) Title() string { return "Issues" }

func (t *IssuesTab) SetSize(width, height int) {
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
	t.inputModal.SetSize(width, height)
}

func (t *IssuesTab) Init() tea.Cmd {
	return tea.Batch(t.fetchIssues(), t.tickRefresh())
}

func (t *IssuesTab) Update(msg tea.Msg) (ui.TabModel, tea.Cmd) {
	// Modal intercept.
	if t.modal.Visible() {
		cmd := t.modal.Update(msg)
		return t, cmd
	}
	if t.inputModal.Visible() {
		cmd := t.inputModal.Update(msg)
		return t, cmd
	}

	switch msg := msg.(type) {
	case messages.IssueListMsg:
		if msg.Err != nil {
			t.notification = fmt.Sprintf("issues: %v", msg.Err)
			return t, nil
		}
		t.issues = nil
		var containers []messages.Container
		for _, issue := range msg.Assigned {
			t.issues = append(t.issues, issue)
			containers = append(containers, issueToContainer(issue, "Assigned to me"))
		}
		for _, issue := range msg.Created {
			if !issueInList(issue, msg.Assigned) {
				t.issues = append(t.issues, issue)
				containers = append(containers, issueToContainer(issue, "Created by me"))
			}
		}
		for _, issue := range msg.Mentioned {
			if !issueInList(issue, msg.Assigned) && !issueInList(issue, msg.Created) {
				t.issues = append(t.issues, issue)
				containers = append(containers, issueToContainer(issue, "Mentioned"))
			}
		}
		t.sidebar.SetItems(containers)
		return t, nil

	case messages.IssueDetailMsg:
		if msg.Err != nil {
			t.notification = fmt.Sprintf("issue detail: %v", msg.Err)
			return t, nil
		}
		detail := gitlabpkg.FormatIssueDetail(msg.Issue, msg.Notes)
		t.detailPane.SetContent(fmt.Sprintf("#%d %s", msg.Issue.IID, msg.Issue.Title), detail)
		return t, nil

	case messages.IssueActionMsg:
		if msg.Err != nil {
			t.notification = fmt.Sprintf("%s failed: %v", msg.Action, msg.Err)
		} else {
			t.notification = msg.Action + " done"
		}
		return t, t.fetchIssues()

	case messages.ExecFinishedMsg:
		return t, nil

	case issueRefreshTickMsg:
		return t, tea.Batch(t.fetchIssues(), t.tickRefresh())

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
				return t, tea.Batch(cmd, t.selectIssue(item.ID))
			}
			return t, cmd
		}
		t.focusSidebar = false
		t.sidebar.SetFocused(false)
		t.detailPane.SetFocused(true)
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

		switch {
		case s == "ctrl+w" || s == "ctrl+W":
			t.pendingCtrlW = true
			return t, nil
		case s == "alt+w" || s == "alt+W":
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

func (t *IssuesTab) updateSidebar(msg tea.KeyPressMsg) (ui.TabModel, tea.Cmd) {
	switch {
	case key.Matches(msg, theme.Keys.Enter):
		if item, ok := t.sidebar.SelectedItem(); ok {
			t.focusSidebar = false
			t.sidebar.SetFocused(false)
			t.detailPane.SetFocused(true)
			return t, t.selectIssue(item.ID)
		}
	case msg.String() == "o":
		// Open in browser.
		if issue := t.findSelectedIssue(); issue != nil && issue.WebURL != "" {
			_ = openBrowser(issue.WebURL)
		}
	case msg.String() == "s":
		// Close/reopen toggle.
		if issue := t.findSelectedIssue(); issue != nil {
			if issue.State == "opened" {
				return t, t.closeIssue(issue.IID)
			}
			return t, t.reopenIssue(issue.IID)
		}
	case msg.String() == "c":
		// Comment — open $EDITOR.
		if issue := t.findSelectedIssue(); issue != nil {
			return t, t.commentOnIssue(issue.IID)
		}
	case msg.String() == "a":
		// Assign to self.
		if issue := t.findSelectedIssue(); issue != nil {
			return t, t.assignToSelf(issue.IID)
		}
	default:
		cmd := t.sidebar.Update(msg)
		return t, cmd
	}
	return t, nil
}

func (t *IssuesTab) View() string {
	sidebarWidth := t.width * 25 / 100
	if sidebarWidth < 30 {
		sidebarWidth = 30
	}
	rightWidth := t.width - sidebarWidth

	left := t.sidebar.View()
	right := t.detailPane.View()

	_ = rightWidth
	return joinHorizontal(left, right, t.height)
}

// Notification implements the Notifier interface.
func (t *IssuesTab) Notification() string {
	n := t.notification
	t.notification = ""
	return n
}

func (t *IssuesTab) toggleFocus() {
	t.focusSidebar = !t.focusSidebar
	t.sidebar.SetFocused(t.focusSidebar)
	t.detailPane.SetFocused(!t.focusSidebar)
}

func (t *IssuesTab) fetchIssues() tea.Cmd {
	return func() tea.Msg {
		assigned, created, mentioned, err := t.client.ListMyIssues()
		return messages.IssueListMsg{
			Assigned:  assigned,
			Created:   created,
			Mentioned: mentioned,
			Err:       err,
		}
	}
}

func (t *IssuesTab) selectIssue(id string) tea.Cmd {
	// Parse IID from the sidebar item ID (format: "123").
	var iid int64
	fmt.Sscanf(id, "%d", &iid) //nolint:errcheck // best effort
	t.selectedIID = iid

	return func() tea.Msg {
		issue, notes, err := t.client.GetIssue(iid)
		return messages.IssueDetailMsg{Issue: issue, Notes: notes, Err: err}
	}
}

func (t *IssuesTab) closeIssue(iid int64) tea.Cmd {
	return func() tea.Msg {
		err := t.client.CloseIssue(iid)
		return messages.IssueActionMsg{Action: "close issue", Err: err}
	}
}

func (t *IssuesTab) reopenIssue(iid int64) tea.Cmd {
	return func() tea.Msg {
		err := t.client.ReopenIssue(iid)
		return messages.IssueActionMsg{Action: "reopen issue", Err: err}
	}
}

func (t *IssuesTab) assignToSelf(iid int64) tea.Cmd {
	return func() tea.Msg {
		err := t.client.AssignIssue(iid, t.client.UserID)
		return messages.IssueActionMsg{Action: "assign to self", Err: err}
	}
}

func (t *IssuesTab) commentOnIssue(iid int64) tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}

	tmpFile := fmt.Sprintf("/tmp/lazydev-comment-%d.md", iid)
	_ = os.WriteFile(tmpFile, []byte(""), 0o600) //nolint:gosec // temp file

	c := exec.Command(editor, tmpFile) //nolint:gosec,noctx // intentional editor open
	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			return messages.IssueActionMsg{Action: "comment", Err: err}
		}
		body, readErr := os.ReadFile(tmpFile) //nolint:gosec // temp file we just created
		if readErr != nil || len(strings.TrimSpace(string(body))) == 0 {
			return messages.IssueActionMsg{Action: "comment", Err: fmt.Errorf("empty comment")}
		}
		postErr := t.client.CommentOnIssue(iid, string(body))
		_ = os.Remove(tmpFile)
		return messages.IssueActionMsg{Action: "comment", Err: postErr}
	})
}

type issueRefreshTickMsg struct{}

func (t *IssuesTab) tickRefresh() tea.Cmd {
	return tea.Tick(time.Duration(t.refreshS)*time.Second, func(time.Time) tea.Msg {
		return issueRefreshTickMsg{}
	})
}

func (t *IssuesTab) findSelectedIssue() *messages.GitLabIssue {
	item, ok := t.sidebar.SelectedItem()
	if !ok {
		return nil
	}
	var iid int64
	fmt.Sscanf(item.ID, "%d", &iid) //nolint:errcheck // best effort
	for i := range t.issues {
		if t.issues[i].IID == iid {
			return &t.issues[i]
		}
	}
	return nil
}

func issueToContainer(issue messages.GitLabIssue, group string) messages.Container {
	state := messages.StateRunning // open = green dot
	if issue.State == "closed" {
		state = messages.StateStopped
	}
	name := fmt.Sprintf("#%d %s", issue.IID, truncate(issue.Title, 40))
	age := relativeTime(issue.UpdatedAt)
	if age != "" {
		name += " " + age
	}
	return messages.Container{
		ID:    fmt.Sprintf("%d", issue.IID),
		Name:  name,
		State: state,
		Group: group,
	}
}

func issueInList(issue messages.GitLabIssue, list []messages.GitLabIssue) bool {
	for _, i := range list {
		if i.IID == issue.IID {
			return true
		}
	}
	return false
}

// openBrowser opens a URL in the default browser.
func openBrowser(url string) error {
	return exec.Command("xdg-open", url).Start() //nolint:gosec // intentional browser open
}

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max-1] + "…"
	}
	return s
}

func relativeTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

// joinHorizontal joins two rendered strings side by side.
func joinHorizontal(left, right string, height int) string {
	leftLines := strings.Split(left, "\n")
	rightLines := strings.Split(right, "\n")

	var b strings.Builder
	for i := 0; i < height; i++ {
		if i < len(leftLines) {
			b.WriteString(leftLines[i])
		}
		if i < len(rightLines) {
			b.WriteString(rightLines[i])
		}
		if i < height-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}
