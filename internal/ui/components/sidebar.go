package components

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/abhishek-rana/lazydk/internal/ui/theme"
	"github.com/abhishek-rana/lazydk/pkg/messages"
)

// SidebarItem represents an item in the sidebar.
type SidebarItem struct {
	ID    string
	Name  string
	State string
	Group string
}

// Sidebar is a grouped list sidebar component.
type Sidebar struct {
	items      []SidebarItem
	groups     []string
	groupItems map[string][]int
	cursor     int
	offset     int
	width      int
	height     int
	focused    bool
}

// NewSidebar creates a new sidebar.
func NewSidebar() Sidebar {
	return Sidebar{
		groupItems: make(map[string][]int),
	}
}

// SetItems updates the sidebar's items.
func (s *Sidebar) SetItems(containers []messages.Container) {
	s.items = make([]SidebarItem, len(containers))
	s.groupItems = make(map[string][]int)
	s.groups = nil

	groupSeen := make(map[string]bool)

	for i, c := range containers {
		group := c.Group
		if group == "" {
			group = "ungrouped"
		}

		s.items[i] = SidebarItem{
			ID:    c.ID,
			Name:  c.Name,
			State: c.Status,
			Group: group,
		}

		if !groupSeen[group] {
			groupSeen[group] = true
			s.groups = append(s.groups, group)
		}
		s.groupItems[group] = append(s.groupItems[group], i)
	}

	if s.cursor >= len(s.items) {
		s.cursor = max(0, len(s.items)-1)
	}
}

// SelectedItem returns the currently selected item, if any.
func (s Sidebar) SelectedItem() (SidebarItem, bool) {
	if len(s.items) == 0 || s.cursor < 0 || s.cursor >= len(s.items) {
		return SidebarItem{}, false
	}
	return s.items[s.cursor], true
}

// SetSize sets the sidebar dimensions.
func (s *Sidebar) SetSize(width, height int) {
	s.width = width
	s.height = height
}

// SetFocused sets whether the sidebar has focus.
func (s *Sidebar) SetFocused(focused bool) {
	s.focused = focused
}

// Focused returns whether the sidebar has focus.
func (s Sidebar) Focused() bool {
	return s.focused
}

// Update handles input for the sidebar.
func (s *Sidebar) Update(msg tea.Msg) tea.Cmd {
	if !s.focused {
		return nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, theme.Keys.Up):
			if s.cursor > 0 {
				s.cursor--
			}
		case key.Matches(msg, theme.Keys.Down):
			if s.cursor < len(s.items)-1 {
				s.cursor++
			}
		}
	}

	return nil
}

// View renders the sidebar.
func (s Sidebar) View() string {
	if len(s.items) == 0 {
		return theme.SidebarStyle.
			Width(s.width).
			Height(s.height).
			Render("  No resources found")
	}

	var b strings.Builder
	lineCount := 0
	visibleHeight := s.height

	if s.cursor < s.offset {
		s.offset = s.cursor
	}
	if s.cursor >= s.offset+visibleHeight {
		s.offset = s.cursor - visibleHeight + 1
	}

	currentLine := 0
	for _, group := range s.groups {
		indices := s.groupItems[group]
		if len(indices) == 0 {
			continue
		}

		if currentLine >= s.offset && lineCount < visibleHeight {
			header := fmt.Sprintf("▼ %s  (%d)", group, len(indices))
			b.WriteString(theme.SidebarGroupStyle.Width(s.width).Render(header))
			b.WriteString("\n")
			lineCount++
		}
		currentLine++

		for _, idx := range indices {
			if currentLine >= s.offset && lineCount < visibleHeight {
				item := s.items[idx]
				icon := theme.StateIcon(item.State)
				name := truncate(item.Name, s.width-6)
				line := fmt.Sprintf("%s %s", icon, name)

				if idx == s.cursor && s.focused {
					b.WriteString(theme.SidebarSelectedStyle.Width(s.width).Render(line))
				} else {
					b.WriteString(theme.SidebarItemStyle.Width(s.width).Render(line))
				}
				b.WriteString("\n")
				lineCount++
			}
			currentLine++
		}
	}

	for lineCount < visibleHeight {
		b.WriteString(strings.Repeat(" ", s.width))
		b.WriteString("\n")
		lineCount++
	}

	return theme.SidebarStyle.Width(s.width).Height(s.height).Render(b.String())
}

func truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
