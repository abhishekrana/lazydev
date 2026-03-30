package components

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/abhishek-rana/lazydev/internal/ui/theme"
)

// TabBar renders a horizontal tab bar.
type TabBar struct {
	Tabs      []string
	ActiveTab int
	Width     int
}

// NewTabBar creates a new tab bar.
func NewTabBar(tabs []string) TabBar {
	return TabBar{
		Tabs: tabs,
	}
}

// View renders the tab bar.
func (t TabBar) View() string {
	var tabs []string
	for i, tab := range t.Tabs {
		if i == t.ActiveTab {
			tabs = append(tabs, theme.ActiveTabStyle.Render(tab))
		} else {
			tabs = append(tabs, theme.InactiveTabStyle.Render(tab))
		}
	}

	row := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)

	gap := ""
	if remaining := t.Width - lipgloss.Width(row); remaining > 0 {
		gap = strings.Repeat(" ", remaining)
	}

	return theme.TabBarStyle.Width(t.Width).Render(row + gap)
}
