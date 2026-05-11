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

// MRsTab displays GitLab merge requests sourced from the local cache.
type MRsTab struct {
	client          *gitlabpkg.Client
	store           *cachepkg.Store
	syncer          *cachepkg.Syncer
	aiUser          string
	sidebar         components.Sidebar
	detailPane      components.DetailPane
	queryline       components.QueryLine
	modal           components.Modal
	focusSidebar    bool
	width           int
	height          int
	selectedIID     int64
	mrs             []messages.GitLabMR
	queryExpr       string
	notification    string
	pendingCtrlW    bool
	fetchSeq        uint64
	pendingFetch    string
	needsAutoSelect bool
}

// NewMRsTab creates a new GitLab Merge Requests tab.
func NewMRsTab(client *gitlabpkg.Client, store *cachepkg.Store, syncer *cachepkg.Syncer, aiUser string) *MRsTab {
	sidebar := components.NewSidebar()
	sidebar.SetFocused(true)
	return &MRsTab{
		client:       client,
		store:        store,
		syncer:       syncer,
		aiUser:       aiUser,
		sidebar:      sidebar,
		detailPane:   components.NewDetailPane(),
		queryline:    components.NewQueryLine(),
		modal:        components.NewModal(),
		focusSidebar: true,
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
	return t.fetchMRs()
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
		var containers []messages.SidebarItem
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
		// Flag auto-select for when the tab becomes active.
		if t.selectedIID == 0 {
			t.needsAutoSelect = true
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
		if t.syncer != nil {
			t.syncer.SyncNow()
		}
		return t, t.fetchMRs()

	case messages.ExecFinishedMsg:
		return t, nil

	case messages.TabActivatedMsg:
		if t.needsAutoSelect {
			t.needsAutoSelect = false
			if item, ok := t.sidebar.SelectedItem(); ok {
				return t, t.selectMR(item.ID)
			}
		}
		return t, nil

	case messages.CacheUpdatedMsg:
		if msg.Kind == "mrs" {
			return t, t.fetchMRs()
		}
		return t, nil

	case messages.ApplyViewMsg:
		t.queryExpr = msg.Expr
		t.notification = "view: " + msg.Name
		return t, t.fetchMRs()

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
		if t.queryline.Visible() {
			esc, cmd := t.queryline.Update(msg)
			if esc {
				t.queryline.Clear()
				t.queryExpr = ""
				return t, t.fetchMRs()
			}
			t.queryExpr = t.queryline.Value()
			return t, tea.Batch(cmd, t.fetchMRs())
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
			return t, t.fetchMRs()
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
	case msg.String() == "R":
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
	if ql := t.queryline.View(); ql != "" {
		view = ql + "\n" + view
	}
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

// fetchMRs reads from the local cache, applying the current queryExpr
// (if any) through the DSL parser. Partitions results into the legacy
// MRListMsg groups so the Update handler keeps its grouping logic.
func (t *MRsTab) fetchMRs() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		env := query.Env{Me: t.client.Username, AI: t.aiUser}
		expr := query.Parse(t.queryExpr, env)

		// If the user narrowed to issues, return empty here — the
		// Issues tab will pick up the same expression.
		if expr.Kind == "issue" {
			return messages.MRListMsg{}
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

		all, err := t.store.ListMRs(ctx, f)
		if err != nil {
			return messages.MRListMsg{Err: err}
		}

		// Structured filters → single flat group.
		if expr.Filter.Assignee != "" || expr.Filter.Author != "" ||
			len(expr.Filter.Labels) > 0 || expr.Filter.Text != "" {
			return messages.MRListMsg{AllOpen: all}
		}

		trackedNames := make(map[string]bool, len(t.client.Usernames))
		for _, u := range t.client.Usernames {
			trackedNames[u] = true
		}
		var mine, review, allOpen []messages.GitLabMR
		seenMine := make(map[int64]bool)
		seenReview := make(map[int64]bool)
		for _, mr := range all {
			if trackedNames[mr.Author] {
				mine = append(mine, mr)
				seenMine[mr.IID] = true
			}
		}
		for _, mr := range all {
			if seenMine[mr.IID] {
				continue
			}
			for _, rv := range mr.Reviewers {
				if trackedNames[rv] {
					review = append(review, mr)
					seenReview[mr.IID] = true
					break
				}
			}
		}
		for _, mr := range all {
			if !seenMine[mr.IID] && !seenReview[mr.IID] {
				allOpen = append(allOpen, mr)
			}
		}
		return messages.MRListMsg{
			Mine:            mine,
			ReviewRequested: review,
			AllOpen:         allOpen,
		}
	}
}

// selectMR paints from the cache instantly, then refreshes from the
// live API in parallel. The later API result overwrites the cached
// paint; both share fetchSeq for staleness rejection.
func (t *MRsTab) selectMR(id string) tea.Cmd {
	var iid int64
	fmt.Sscanf(id, "%d", &iid) //nolint:errcheck,gosec // best effort
	t.selectedIID = iid
	t.fetchSeq++
	seq := t.fetchSeq

	cacheCmd := func() tea.Msg {
		ctx := context.Background()
		cached, notes, err := t.store.GetMR(ctx, iid)
		if err != nil || cached == nil {
			return nil
		}
		return mrDetailResultMsg{seq: seq, mr: *cached, notes: notes}
	}
	apiCmd := func() tea.Msg {
		mr, notes, err := t.client.GetMR(iid)
		if err == nil {
			ctx := context.Background()
			_ = t.store.UpsertMRs(ctx, []messages.GitLabMR{mr})
			_ = t.store.UpsertNotes(ctx, "mr", iid, notes)
		}
		return mrDetailResultMsg{seq: seq, mr: mr, notes: notes, err: err}
	}
	return tea.Batch(cacheCmd, apiCmd)
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

type mrDetailFetchMsg struct{ itemID string }

type mrDetailResultMsg struct {
	seq   uint64
	mr    messages.GitLabMR
	notes []messages.GitLabNote
	err   error
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

func mrToContainer(mr messages.GitLabMR, group string) messages.SidebarItem {
	state := messages.StateOpen
	switch mr.State {
	case "merged":
		state = messages.StateMerged
	case "closed":
		state = messages.StateClosed
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
	return messages.SidebarItem{
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
