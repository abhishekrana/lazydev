package components

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/abhishek-rana/lazydk/internal/ui/theme"
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
				{"Enter", "Select / collapse group"},
				{"Esc", "Back / cancel"},
				{"gg", "Scroll to top"},
				{"G", "Scroll to bottom (auto-follow)"},
			},
		},
		{
			title: "Logs",
			keys: [][2]string{
				{"/", "Search logs"},
				{"f", "Cycle log level filter"},
				{"w", "Toggle line wrap on/off"},
				{"y", "Yank current line to clipboard"},
				{"Y", "Yank all filtered lines to clipboard"},
				{"e", "Export filtered logs to text file"},
				{"E", "Export filtered logs to JSON file"},
			},
		},
		{
			title: "Actions",
			keys: [][2]string{
				{"r", "Restart container/pod"},
				{"s", "Stop container"},
				{"d", "Delete container/pod"},
				{"D", "Describe / inspect (toggle)"},
				{"y", "View YAML (K8s)"},
				{"x", "Exec shell"},
				{"p", "Port forward (K8s)"},
				{"S", "Scale deployment (K8s)"},
			},
		},
		{
			title: "Dashboard",
			keys: [][2]string{
				{"1-6", "Sort by column"},
			},
		},
	}

	var b strings.Builder

	title := theme.ModalTitleStyle.Render("  lazydk — Keybindings  ")
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
