package components

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/abhishek-rana/lazydev/internal/ui/theme"
)

// HelpOverlay displays a full-screen keybinding reference.
type HelpOverlay struct {
	visible bool
	width   int
	height  int
}

// NewHelpOverlay creates a new help overlay (initially hidden).
func NewHelpOverlay() HelpOverlay {
	return HelpOverlay{}
}

// Toggle shows/hides the help overlay.
func (h *HelpOverlay) Toggle() {
	h.visible = !h.visible
}

// Hide dismisses the overlay.
func (h *HelpOverlay) Hide() {
	h.visible = false
}

// Visible returns whether the overlay is shown.
func (h HelpOverlay) Visible() bool {
	return h.visible
}

// SetSize sets terminal size.
func (h *HelpOverlay) SetSize(width, height int) {
	h.width = width
	h.height = height
}

// Update handles input.
func (h *HelpOverlay) Update(msg tea.Msg) tea.Cmd {
	if !h.visible {
		return nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "?", "esc", "q": //nolint:goconst // idiomatic key names
			h.Hide()
			return nil
		}
	}

	return nil
}

type helpSection struct {
	title string
	keys  [][2]string // [key, description]
}

// View renders the help overlay.
func (h HelpOverlay) View() string {
	if !h.visible {
		return ""
	}

	sections := []helpSection{
		{
			title: "Global",
			keys: [][2]string{
				{"q / Ctrl+C", "Quit"},
				{"1-9", "Switch to tab by number"},
				{"Tab", "Next tab"},
				{"Shift+Tab", "Previous tab"},
				{"?", "Toggle help"},
				{":", "Command palette"},
			},
		},
		{
			title: "Navigation",
			keys: [][2]string{
				{"j / ↓", "Move down"},
				{"k / ↑", "Move up"},
				{"Ctrl+W W", "Switch pane focus"},
				{"Alt+W", "Switch pane focus"},
				{"Enter", "Select / collapse group"},
				{"Esc", "Back / cancel"},
				{"gg", "Scroll to top"},
				{"G", "Scroll to bottom (auto-follow)"},
			},
		},
		{
			title: "Query (DSL on /)",
			keys: [][2]string{
				{"/", "Query line: assignee:@me label:bug state:open …"},
				{"r", "Refresh now (sync nudge)"},
				{"1–9", "Recall saved view (mine, ai-queue, …)"},
				{":save <n> <e>", "Save current view"},
			},
		},
		{
			title: "Select & export",
			keys: [][2]string{
				{"Space", "Mark current item"},
				{"v", "Visual range (toggle)"},
				{"Esc", "Clear marks"},
				{"y", "Copy marked → clipboard (OSC52)"},
				{"Y", "Write marked → /tmp/lazydev-ctx-*.md"},
				{"X", "Pipe marked → llm_command (claude -p)"},
			},
		},
		{
			title: "Issues",
			keys: [][2]string{
				{"Enter", "Open detail"},
				{"s", "Close / reopen"},
				{"c", "Comment (opens $EDITOR)"},
				{"a", "Assign to self"},
				{"T", "Toggle assignee self ↔ ai_user"},
				{"N", "Quick-create assigned to ai_user"},
				{"o", "Open in browser"},
			},
		},
		{
			title: "Merge Requests",
			keys: [][2]string{
				{"Enter", "Open detail"},
				{"R", "Review in neovim (DiffviewOpen)"},
				{"m", "Merge (with confirmation)"},
				{"A", "Approve"},
				{"T", "Toggle assignee self ↔ ai_user"},
				{"s", "Close / reopen"},
				{"c", "Comment (opens $EDITOR)"},
				{"o", "Open in browser"},
			},
		},
	}

	var b strings.Builder

	title := theme.ModalTitleStyle.Render("  lazydev — Keybindings  ")
	b.WriteString(title)
	b.WriteString("\n\n")

	for _, sec := range sections {
		b.WriteString(theme.SidebarGroupStyle.Render(sec.title))
		b.WriteString("\n")
		for _, kv := range sec.keys {
			keyStr := theme.StatusBarKeyStyle.Render(padRight(kv[0], 16))
			b.WriteString("  ")
			b.WriteString(keyStr)
			b.WriteString("  ")
			b.WriteString(kv[1])
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	b.WriteString(theme.LogTimestampStyle.Render("Press ? or Esc to close"))

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorPrimary).
		Padding(1, 3).
		Render(b.String())

	return lipgloss.Place(h.width, h.height, lipgloss.Center, lipgloss.Center, box)
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
