package tabs

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/abhishek-rana/lazydev/internal/claude"
	"github.com/abhishek-rana/lazydev/internal/ui"
	"github.com/abhishek-rana/lazydev/internal/ui/theme"
	"github.com/abhishek-rana/lazydev/pkg/messages"
)

// ClaudeTab lists Claude Code sessions persisted in
// `.lazydev/sessions.json`. Enter re-attaches an interactive session
// via tmux; `d` drops the record; `o` opens the originating issue/MR.
type ClaudeTab struct {
	opts         *Options
	sessions     []claude.Session
	cursor       int
	width        int
	height       int
	notification string
	loadErr      error
}

// NewClaudeTab constructs the sessions list tab.
func NewClaudeTab(opts *Options) *ClaudeTab {
	if opts == nil {
		opts = &Options{}
	}
	return &ClaudeTab{opts: opts}
}

func (t *ClaudeTab) Title() string { return "Claude" }

func (t *ClaudeTab) SetSize(width, height int) {
	t.width = width
	t.height = height
}

func (t *ClaudeTab) Init() tea.Cmd {
	return t.reload()
}

func (t *ClaudeTab) Update(msg tea.Msg) (ui.TabModel, tea.Cmd) {
	switch msg := msg.(type) {
	case messages.TabActivatedMsg, messages.ClaudeSessionsReloadMsg, messages.ClaudeDispatchMsg:
		return t, t.reload()

	case sessionsLoadedMsg:
		t.sessions = msg.sessions
		t.loadErr = msg.err
		if t.cursor >= len(t.sessions) {
			t.cursor = len(t.sessions) - 1
		}
		if t.cursor < 0 {
			t.cursor = 0
		}
		return t, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "j", "down":
			if t.cursor < len(t.sessions)-1 {
				t.cursor++
			}
			return t, nil
		case "k", "up":
			if t.cursor > 0 {
				t.cursor--
			}
			return t, nil
		case "g":
			t.cursor = 0
			return t, nil
		case "G":
			t.cursor = len(t.sessions) - 1
			if t.cursor < 0 {
				t.cursor = 0
			}
			return t, nil
		case "r":
			return t, t.reload()
		case "enter":
			return t, t.attach()
		case "o":
			return t, t.openRef()
		case "d":
			return t, t.delete()
		case "L":
			return t, t.openLog()
		}
	}
	return t, nil
}

func (t *ClaudeTab) View() string {
	if !t.opts.ClaudeEnv.ClaudeAvailable() {
		return t.renderEmpty("Claude Code not detected on PATH. Install: https://docs.anthropic.com/claude-code")
	}
	if t.opts.ClaudeStore == nil {
		return t.renderEmpty("No repo root detected — sessions cannot be persisted.")
	}
	if t.loadErr != nil {
		return t.renderEmpty(fmt.Sprintf("Error loading sessions: %v", t.loadErr))
	}
	if len(t.sessions) == 0 {
		hint := "Press 'C' on an issue or MR to start an interactive Claude session,\n" +
			"or 'P' for a one-shot `claude -p` run."
		return t.renderEmpty(hint)
	}

	var b strings.Builder
	header := theme.InactiveHeaderStyle.Render(fmt.Sprintf(
		"%-10s %-6s %-12s %-5s %-12s %-9s %s",
		"ID", "KIND", "REF", "MODE", "LAST SEEN", "STATUS", "TITLE",
	))
	b.WriteString(header)
	b.WriteString("\n")

	for i, s := range t.sessions {
		line := fmt.Sprintf(
			"%-10s %-6s %-12s %-5s %-12s %-9s %s",
			s.ID[:8],
			s.Kind,
			s.Ref,
			shortMode(s.Mode),
			relativeTime(s.LastSeenAt),
			s.Status,
			truncate(s.Title, max0(t.width-60)),
		)
		if i == t.cursor {
			line = theme.SidebarSelectedStyle.Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	footer := theme.StatusBarStyle.Render(
		"[enter] attach  [o] open ref  [L] tail log  [d] drop record  [r] reload",
	)
	b.WriteString("\n")
	b.WriteString(footer)
	return b.String()
}

// Notification implements ui.Notifier.
func (t *ClaudeTab) Notification() string {
	n := t.notification
	t.notification = ""
	return n
}

func (t *ClaudeTab) renderEmpty(msg string) string {
	style := lipgloss.NewStyle().
		Width(t.width).
		Height(t.height).
		Align(lipgloss.Center, lipgloss.Center)
	return style.Render(msg)
}

func (t *ClaudeTab) reload() tea.Cmd {
	store := t.opts.ClaudeStore
	if store == nil {
		return nil
	}
	return func() tea.Msg {
		list, err := store.List()
		return sessionsLoadedMsg{sessions: list, err: err}
	}
}

func (t *ClaudeTab) attach() tea.Cmd {
	sess, ok := t.selected()
	if !ok {
		return nil
	}
	if sess.Mode != claude.ModeInteractive {
		t.notification = "one-shot sessions can't be attached (use L to tail the log)"
		return nil
	}
	c := claude.AttachCommand(t.opts.ClaudeEnv, sess)
	if c == nil {
		t.notification = "no tmux target on this session"
		return nil
	}
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return messages.ExecFinishedMsg{Err: err}
	})
}

func (t *ClaudeTab) openRef() tea.Cmd {
	// We don't have the URL stored on the session — best-effort no-op
	// for now. Future: stash URL in Session struct on dispatch.
	t.notification = "open-ref not yet implemented (session record lacks URL)"
	return nil
}

func (t *ClaudeTab) delete() tea.Cmd {
	sess, ok := t.selected()
	if !ok {
		return nil
	}
	id := sess.ID
	store := t.opts.ClaudeStore
	return func() tea.Msg {
		_ = store.Delete(id)
		return messages.ClaudeSessionsReloadMsg{}
	}
}

func (t *ClaudeTab) openLog() tea.Cmd {
	sess, ok := t.selected()
	if !ok {
		return nil
	}
	if sess.LogPath == "" {
		t.notification = "session has no log file"
		return nil
	}
	pager := "less"
	c := exec.Command(pager, sess.LogPath) //nolint:gosec,noctx // user-driven log tail
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return messages.ExecFinishedMsg{Err: err}
	})
}

func (t *ClaudeTab) selected() (claude.Session, bool) {
	if t.cursor < 0 || t.cursor >= len(t.sessions) {
		return claude.Session{}, false
	}
	return t.sessions[t.cursor], true
}

func shortMode(m claude.Mode) string {
	switch m {
	case claude.ModeInteractive:
		return "int"
	case claude.ModeOneShot:
		return "one"
	default:
		return string(m)
	}
}

// max0 returns x if positive, else 0. Avoids passing a negative width
// to truncate when the terminal is narrow.
func max0(x int) int {
	if x < 0 {
		return 0
	}
	return x
}

// sessionsLoadedMsg is the local result of reload().
type sessionsLoadedMsg struct {
	sessions []claude.Session
	err      error
}

// Ensure relativeTime + truncate are in the package (they are — defined
// in issues.go). Local imports above keep the file self-contained.
var _ = time.Time{}
