package components

import (
	"fmt"

	"charm.land/lipgloss/v2"
	"github.com/abhishek-rana/lazydk/internal/ui/theme"
)

// StatusBar renders the bottom status bar.
type StatusBar struct {
	Width   int
	Context string
	Message string
}

// NewStatusBar creates a new status bar.
func NewStatusBar() StatusBar {
	return StatusBar{}
}

// View renders the status bar.
func (s StatusBar) View() string {
	keys := fmt.Sprintf(
		"%s quit  %s search  %s cmd  %s help",
		theme.StatusBarKeyStyle.Render("[q]"),
		theme.StatusBarKeyStyle.Render("[/]"),
		theme.StatusBarKeyStyle.Render("[:]"),
		theme.StatusBarKeyStyle.Render("[?]"),
	)

	right := ""
	if s.Context != "" {
		right = fmt.Sprintf("ctx: %s", s.Context)
	}

	gap := s.Width - lipgloss.Width(keys) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}

	content := keys + fmt.Sprintf("%*s%s", gap, "", right)

	return theme.StatusBarStyle.Width(s.Width).Render(content)
}
