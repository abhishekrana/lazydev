package components

import (
	"fmt"

	"charm.land/lipgloss/v2"
	"github.com/abhishek-rana/lazydev/internal/ui/theme"
)

// StatusBar renders the bottom status bar.
type StatusBar struct {
	Width   int
	Context string
	Message string
	Sync    string // sync indicator, e.g. "prefetching 120" or "synced 5s ago"
	// SyncTone influences the rendered colour of Sync — "" (default
	// neutral), "ok" (green), "warn" (yellow), "err" (red).
	SyncTone string
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

	parts := make([]string, 0, 2)
	if s.Sync != "" {
		parts = append(parts, renderSync(s.Sync, s.SyncTone))
	}
	if s.Context != "" {
		parts = append(parts, fmt.Sprintf("ctx: %s", s.Context))
	}
	right := ""
	for i, p := range parts {
		if i > 0 {
			right += "  "
		}
		right += p
	}

	gap := s.Width - lipgloss.Width(keys) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}

	content := keys + fmt.Sprintf("%*s%s", gap, "", right)

	return theme.StatusBarStyle.Width(s.Width).Render(content)
}

func renderSync(text, tone string) string {
	switch tone {
	case "ok":
		return theme.LogInfoStyle.Render(text)
	case "warn":
		return theme.LogWarnStyle.Render(text)
	case "err":
		return theme.LogErrorStyle.Render(text)
	default:
		return text
	}
}
