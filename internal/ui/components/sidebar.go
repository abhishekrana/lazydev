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

// displayRow represents a row in the flattened sidebar view.
type displayRow struct {
	isGroup bool
	group   string
	itemIdx int // index into s.items, only valid when !isGroup
}

// Sidebar is a grouped list sidebar component with collapsible groups.
type Sidebar struct {
	items      []SidebarItem
	groups     []string
	groupItems map[string][]int
	collapsed  map[string]bool
	rows       []displayRow // flattened display rows
	cursor     int          // index into rows
	offset     int
	width      int
	height     int
	focused    bool
	yOffset    int // screen Y offset (e.g. tab bar height)
}

// NewSidebar creates a new sidebar.
func NewSidebar() Sidebar {
	return Sidebar{
		groupItems: make(map[string][]int),
		collapsed:  make(map[string]bool),
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

	s.rebuildRows()

	if s.cursor >= len(s.rows) {
		s.cursor = max(0, len(s.rows)-1)
	}
}

// rebuildRows flattens groups + items into display rows respecting collapsed state.
func (s *Sidebar) rebuildRows() {
	s.rows = nil
	for _, group := range s.groups {
		indices := s.groupItems[group]
		if len(indices) == 0 {
			continue
		}
		s.rows = append(s.rows, displayRow{isGroup: true, group: group})
		if !s.collapsed[group] {
			for _, idx := range indices {
				s.rows = append(s.rows, displayRow{isGroup: false, group: group, itemIdx: idx})
			}
		}
	}
}

// SelectedItem returns the currently selected item, if any.
// Returns false if cursor is on a group header or no items exist.
func (s Sidebar) SelectedItem() (SidebarItem, bool) {
	if len(s.rows) == 0 || s.cursor < 0 || s.cursor >= len(s.rows) {
		return SidebarItem{}, false
	}
	row := s.rows[s.cursor]
	if row.isGroup {
		return SidebarItem{}, false
	}
	if row.itemIdx < 0 || row.itemIdx >= len(s.items) {
		return SidebarItem{}, false
	}
	return s.items[row.itemIdx], true
}

// SetSize sets the sidebar dimensions.
func (s *Sidebar) SetSize(width, height int) {
	s.width = width
	s.height = height
}

// SetYOffset sets the screen Y offset for mouse click mapping (e.g. tab bar height).
func (s *Sidebar) SetYOffset(offset int) {
	s.yOffset = offset
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
	switch msg := msg.(type) {
	case tea.MouseClickMsg:
		mouse := msg.Mouse()
		// Map click Y to row index, adjusting for screen offset (tab bar).
		clickedRow := mouse.Y - s.yOffset + s.offset
		if clickedRow >= 0 && clickedRow < len(s.rows) {
			s.cursor = clickedRow
			// Toggle group on click.
			if s.rows[clickedRow].isGroup {
				group := s.rows[clickedRow].group
				s.collapsed[group] = !s.collapsed[group]
				s.rebuildRows()
				if s.cursor >= len(s.rows) {
					s.cursor = len(s.rows) - 1
				}
			}
		}
		return nil

	case tea.KeyPressMsg:
		if !s.focused {
			return nil
		}
		switch {
		case key.Matches(msg, theme.Keys.Up):
			if s.cursor > 0 {
				s.cursor--
			}
		case key.Matches(msg, theme.Keys.Down):
			if s.cursor < len(s.rows)-1 {
				s.cursor++
			}
		case key.Matches(msg, theme.Keys.Enter):
			// Toggle collapse on group headers.
			if s.cursor >= 0 && s.cursor < len(s.rows) && s.rows[s.cursor].isGroup {
				group := s.rows[s.cursor].group
				s.collapsed[group] = !s.collapsed[group]
				s.rebuildRows()
				if s.cursor >= len(s.rows) {
					s.cursor = len(s.rows) - 1
				}
			}
		}
	}

	return nil
}

// View renders the sidebar.
func (s Sidebar) View() string {
	if len(s.rows) == 0 {
		return theme.SidebarStyle.
			Width(s.width).
			Height(s.height).
			Render("  No resources found")
	}

	var b strings.Builder
	visibleHeight := s.height

	// Ensure cursor is visible.
	if s.cursor < s.offset {
		s.offset = s.cursor
	}
	if s.cursor >= s.offset+visibleHeight {
		s.offset = s.cursor - visibleHeight + 1
	}

	end := s.offset + visibleHeight
	if end > len(s.rows) {
		end = len(s.rows)
	}

	lineCount := 0
	for i := s.offset; i < end; i++ {
		row := s.rows[i]

		if row.isGroup {
			arrow := "▼"
			if s.collapsed[row.group] {
				arrow = "▶"
			}
			count := len(s.groupItems[row.group])
			header := fmt.Sprintf("%s %s  (%d)", arrow, row.group, count)

			if i == s.cursor && s.focused {
				b.WriteString(theme.SidebarSelectedStyle.Width(s.width).Render(header))
			} else {
				b.WriteString(theme.SidebarGroupStyle.Width(s.width).Render(header))
			}
		} else {
			item := s.items[row.itemIdx]
			icon := theme.StateIcon(item.State)
			name := truncate(item.Name, s.width-6)
			line := fmt.Sprintf("%s %s", icon, name)

			if i == s.cursor && s.focused {
				b.WriteString(theme.SidebarSelectedStyle.Width(s.width).Render(line))
			} else {
				b.WriteString(theme.SidebarItemStyle.Width(s.width).Render(line))
			}
		}
		b.WriteString("\n")
		lineCount++
	}

	// Pad remaining space.
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
