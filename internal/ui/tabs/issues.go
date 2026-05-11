package tabs

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	cachepkg "github.com/abhishek-rana/lazydev/internal/cache"
	gitlabpkg "github.com/abhishek-rana/lazydev/internal/gitlab"
	"github.com/abhishek-rana/lazydev/internal/query"
	"github.com/abhishek-rana/lazydev/internal/ui"
	"github.com/abhishek-rana/lazydev/internal/ui/components"
	"github.com/abhishek-rana/lazydev/internal/ui/theme"
	"github.com/abhishek-rana/lazydev/pkg/messages"
)

// IssuesTab displays GitLab issues sourced from the local cache.
type IssuesTab struct {
	client          *gitlabpkg.Client
	store           *cachepkg.Store
	syncer          *cachepkg.Syncer
	aiUser          string
	sidebar         components.Sidebar
	detailPane      components.DetailPane
	queryline       components.QueryLine
	modal           components.Modal
	inputModal      components.InputModal
	focusSidebar    bool
	width           int
	height          int
	selectedIID     int64
	issues          []messages.GitLabIssue // flat list for lookup
	queryExpr       string
	notification    string
	pendingCtrlW    bool
	fetchSeq        uint64 // detail-fetch staleness counter
	pendingFetch    string // sidebar item ID waiting for debounce
	needsAutoSelect bool
}

// NewIssuesTab creates a new GitLab Issues tab. The cache is the
// source of truth for reads; the syncer is nudged on manual refresh.
// aiUser is the username `@ai` resolves to in the query DSL.
func NewIssuesTab(client *gitlabpkg.Client, store *cachepkg.Store, syncer *cachepkg.Syncer, aiUser string) *IssuesTab {
	sidebar := components.NewSidebar()
	sidebar.SetFocused(true)
	return &IssuesTab{
		client:       client,
		store:        store,
		syncer:       syncer,
		aiUser:       aiUser,
		sidebar:      sidebar,
		detailPane:   components.NewDetailPane(),
		queryline:    components.NewQueryLine(),
		modal:        components.NewModal(),
		inputModal:   components.NewInputModal(),
		focusSidebar: true,
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
	// Sidebar populates from the cache before any network call.
	// The syncer (started by main.go) will emit CacheUpdatedMsg later
	// to refresh the view as fresh data arrives.
	return t.fetchIssues()
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
		var containers []messages.SidebarItem
		seenInSprint := make(map[int64]bool)

		// Current Sprint group: issues belonging to the current iteration.
		if msg.CurrentIteration != nil {
			// Build the sprint group label.
			sprintGroup := "Current Sprint"
			if msg.CurrentIteration.Title != "" {
				sprintGroup += ": " + msg.CurrentIteration.Title
			}
			if !msg.CurrentIteration.Due.IsZero() {
				sprintGroup += fmt.Sprintf(" (due %s)", msg.CurrentIteration.Due.Format("Jan 2"))
			}

			iterID := msg.CurrentIteration.ID
			allIssues := append(append(msg.Assigned, msg.Created...), msg.Mentioned...)
			for _, issue := range allIssues {
				if issueInIteration(issue, iterID) && !seenInSprint[issue.IID] {
					seenInSprint[issue.IID] = true
					t.issues = append(t.issues, issue)
					containers = append(containers, issueToContainer(issue, sprintGroup))
				}
			}
		}

		seenBacklog := make(map[int64]bool)
		for _, issue := range msg.Assigned {
			if !seenInSprint[issue.IID] && !seenBacklog[issue.IID] {
				seenBacklog[issue.IID] = true
				t.issues = append(t.issues, issue)
				containers = append(containers, issueToContainer(issue, "Backlog"))
			}
		}
		for _, issue := range msg.Created {
			if !seenInSprint[issue.IID] && !seenBacklog[issue.IID] {
				seenBacklog[issue.IID] = true
				t.issues = append(t.issues, issue)
				containers = append(containers, issueToContainer(issue, "Backlog"))
			}
		}
		for _, issue := range msg.Mentioned {
			if !seenInSprint[issue.IID] && !seenBacklog[issue.IID] {
				seenBacklog[issue.IID] = true
				t.issues = append(t.issues, issue)
				containers = append(containers, issueToContainer(issue, "Mentioned"))
			}
		}
		t.sidebar.SetItems(containers)
		// Flag auto-select for when the tab becomes active.
		if t.selectedIID == 0 {
			t.needsAutoSelect = true
		}
		return t, nil

	case issueDetailFetchMsg:
		// Only fetch if this is still the pending item (debounce).
		if msg.itemID == t.pendingFetch {
			t.detailPane.SetContent("Loading...", "Fetching issue details...")
			return t, t.selectIssue(msg.itemID)
		}
		return t, nil

	case issueDetailResultMsg:
		// Discard stale responses.
		if msg.seq != t.fetchSeq {
			return t, nil
		}
		if msg.err != nil {
			t.notification = fmt.Sprintf("issue detail: %v", msg.err)
			return t, nil
		}
		detail := gitlabpkg.FormatIssueDetail(msg.issue, msg.notes, msg.relatedMRs, t.detailPane.Width())
		t.detailPane.SetContent(fmt.Sprintf("#%d %s", msg.issue.IID, msg.issue.Title), detail)
		return t, nil

	case messages.IssueActionMsg:
		if msg.Err != nil {
			t.notification = fmt.Sprintf("%s failed: %v", msg.Action, msg.Err)
		} else {
			t.notification = msg.Action + " done"
		}
		// Action committed → nudge syncer so the cache reflects the
		// new state on the next tick.
		if t.syncer != nil {
			t.syncer.SyncNow()
		}
		return t, t.fetchIssues()

	case messages.ExecFinishedMsg:
		return t, nil

	case messages.TabActivatedMsg:
		if t.needsAutoSelect {
			t.needsAutoSelect = false
			if item, ok := t.sidebar.SelectedItem(); ok {
				return t, t.selectIssue(item.ID)
			}
		}
		return t, nil

	case messages.CacheUpdatedMsg:
		if msg.Kind == "issues" {
			return t, t.fetchIssues()
		}
		return t, nil

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
		// Queryline intercepts all keys while visible.
		if t.queryline.Visible() {
			esc, cmd := t.queryline.Update(msg)
			if esc {
				t.queryline.Clear()
				t.queryExpr = ""
				return t, t.fetchIssues()
			}
			t.queryExpr = t.queryline.Value()
			return t, tea.Batch(cmd, t.fetchIssues())
		}

		s := msg.String()
		if t.pendingCtrlW {
			t.pendingCtrlW = false
			if s == "w" || s == "W" || s == "ctrl+w" || s == "ctrl+W" { //nolint:goconst // key names
				t.toggleFocus()
				return t, nil
			}
		}

		switch s {
		case "/":
			t.queryline.Show()
			return t, nil
		case "r":
			if t.syncer != nil {
				t.syncer.SyncNow()
				t.notification = "Refreshing..."
			}
			return t, t.fetchIssues()
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

func (t *IssuesTab) updateSidebar(msg tea.KeyPressMsg) (ui.TabModel, tea.Cmd) {
	switch {
	case key.Matches(msg, theme.Keys.Enter):
		if item, ok := t.sidebar.SelectedItem(); ok {
			t.focusSidebar = false
			t.sidebar.SetFocused(false)
			t.detailPane.SetFocused(true)
			t.detailPane.SetContent("Loading...", "Fetching issue details...")
			return t, t.selectIssue(item.ID)
		}
	case msg.String() == "o":
		if issue := t.findSelectedIssue(); issue != nil && issue.WebURL != "" {
			_ = openBrowser(issue.WebURL)
		}
	case msg.String() == "s":
		if issue := t.findSelectedIssue(); issue != nil {
			if issue.State == "opened" {
				iid := issue.IID
				t.modal.Show("Close Issue", fmt.Sprintf("Close #%d %s?", issue.IID, issue.Title), func() tea.Cmd {
					return t.closeIssue(iid)
				})
			} else {
				iid := issue.IID
				t.modal.Show("Reopen Issue", fmt.Sprintf("Reopen #%d %s?", issue.IID, issue.Title), func() tea.Cmd {
					return t.reopenIssue(iid)
				})
			}
			return t, nil
		}
	case msg.String() == "c":
		if issue := t.findSelectedIssue(); issue != nil {
			return t, t.commentOnIssue(issue.IID)
		}
	case msg.String() == "a":
		if issue := t.findSelectedIssue(); issue != nil {
			iid := issue.IID
			t.modal.Show("Assign Issue", fmt.Sprintf("Assign #%d to yourself?", issue.IID), func() tea.Cmd {
				return t.assignToSelf(iid)
			})
			return t, nil
		}
	default:
		prevItem, _ := t.sidebar.SelectedItem()
		cmd := t.sidebar.Update(msg)
		// Debounce detail fetch when cursor moves to a different item.
		if newItem, ok := t.sidebar.SelectedItem(); ok && newItem.ID != prevItem.ID {
			t.pendingFetch = newItem.ID
			return t, tea.Batch(cmd, tea.Tick(150*time.Millisecond, func(time.Time) tea.Msg {
				return issueDetailFetchMsg{itemID: newItem.ID}
			}))
		}
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
	view := joinHorizontal(left, right, t.height)
	if ql := t.queryline.View(); ql != "" {
		view = ql + "\n" + view
	}
	if t.modal.Visible() {
		return t.modal.View()
	}
	return view
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

// fetchIssues reads from the local cache, applying the current
// queryExpr (if any) parsed via the DSL. The result is shaped into
// the legacy IssueListMsg so the Update handler keeps its grouping
// logic.
func (t *IssuesTab) fetchIssues() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		env := query.Env{Me: t.client.Username, AI: t.aiUser}
		expr := query.Parse(t.queryExpr, env)

		// If the user explicitly narrowed to MRs, return empty here —
		// the MRs tab will pick up the same expression.
		if expr.Kind == "mr" {
			return messages.IssueListMsg{}
		}

		f := expr.Filter
		if f.State == "" {
			f.State = "open"
		}
		if f.Limit == 0 {
			f.Limit = 1000
		}
		if !expr.UpdatedAfter.IsZero() {
			f.UpdatedAfter = expr.UpdatedAfter
		}
		if !expr.UpdatedBefore.IsZero() {
			f.UpdatedBefore = expr.UpdatedBefore
		}

		all, err := t.store.ListIssues(ctx, f)
		if err != nil {
			return messages.IssueListMsg{Err: err}
		}

		// If the user filtered by a structured field, return one flat
		// group: the partition heuristic ("Mine" vs "All") only makes
		// sense for an unfiltered view.
		if expr.Filter.Assignee != "" || expr.Filter.Author != "" ||
			len(expr.Filter.Labels) > 0 || expr.Filter.Text != "" {
			return messages.IssueListMsg{Created: all}
		}

		trackedNames := make(map[string]bool, len(t.client.Usernames))
		for _, u := range t.client.Usernames {
			trackedNames[u] = true
		}
		var assigned, created []messages.GitLabIssue
		seen := make(map[int64]bool)
		for _, iss := range all {
			if trackedNames[iss.Assignee] {
				assigned = append(assigned, iss)
				seen[iss.IID] = true
			}
		}
		for _, iss := range all {
			if seen[iss.IID] {
				continue
			}
			if trackedNames[iss.Author] {
				created = append(created, iss)
			}
		}
		return messages.IssueListMsg{
			Assigned: assigned,
			Created:  created,
		}
	}
}

// selectIssue paints from the cache instantly, then refreshes from the
// live API in parallel. The later API result overwrites the cached
// paint when it arrives (typically 1–2s later); both share the same
// fetchSeq so staleness checks reject results from earlier selections.
func (t *IssuesTab) selectIssue(id string) tea.Cmd {
	var iid int64
	fmt.Sscanf(id, "%d", &iid) //nolint:errcheck,gosec // best effort
	t.selectedIID = iid
	t.fetchSeq++
	seq := t.fetchSeq

	cacheCmd := func() tea.Msg {
		ctx := context.Background()
		cached, notes, related, err := t.store.GetIssue(ctx, iid)
		if err != nil || cached == nil {
			return nil
		}
		return issueDetailResultMsg{seq: seq, issue: *cached, notes: notes, relatedMRs: related}
	}
	apiCmd := func() tea.Msg {
		issue, notes, related, err := t.client.GetIssue(iid)
		if err == nil {
			ctx := context.Background()
			_ = t.store.UpsertIssues(ctx, []messages.GitLabIssue{issue})
			_ = t.store.UpsertNotes(ctx, "issue", iid, notes)
			_ = t.store.UpsertRelatedMRs(ctx, iid, related)
		}
		return issueDetailResultMsg{seq: seq, issue: issue, notes: notes, relatedMRs: related, err: err}
	}
	return tea.Batch(cacheCmd, apiCmd)
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

// issueDetailFetchMsg is a debounced trigger to fetch issue details.
type issueDetailFetchMsg struct {
	itemID string
}

// issueDetailResultMsg wraps the cache- or API-sourced detail with a
// sequence number for staleness check. Two results per click are
// expected (cache first, API second); both pass the same `seq`.
type issueDetailResultMsg struct {
	seq        uint64
	issue      messages.GitLabIssue
	notes      []messages.GitLabNote
	relatedMRs []messages.GitLabIssueMR
	err        error
}

func (t *IssuesTab) findSelectedIssue() *messages.GitLabIssue {
	item, ok := t.sidebar.SelectedItem()
	if !ok {
		return nil
	}
	var iid int64
	fmt.Sscanf(item.ID, "%d", &iid) //nolint:errcheck,gosec // best effort
	for i := range t.issues {
		if t.issues[i].IID == iid {
			return &t.issues[i]
		}
	}
	return nil
}

func issueToContainer(issue messages.GitLabIssue, group string) messages.SidebarItem {
	state := messages.StateOpen
	if issue.State == "closed" {
		state = messages.StateClosed
	}
	name := fmt.Sprintf("#%d %s", issue.IID, truncate(issue.Title, 40))
	age := relativeTime(issue.UpdatedAt)
	if age != "" {
		name += " " + age
	}
	return messages.SidebarItem{
		ID:    fmt.Sprintf("%d", issue.IID),
		Name:  name,
		State: state,
		Group: group,
	}
}

func issueInIteration(issue messages.GitLabIssue, iterationID int64) bool {
	return issue.IterationID == iterationID && iterationID != 0
}

// openBrowser opens a URL in the default browser.
func openBrowser(url string) error {
	return exec.Command("xdg-open", url).Start() //nolint:gosec,noctx // intentional browser open
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
