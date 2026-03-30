package components

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/abhishek-rana/lazydk/internal/ui/theme"
	"github.com/abhishek-rana/lazydk/pkg/messages"
)

// SidebarItem represents an item in the sidebar.
type SidebarItem struct {
	ID    string
	Name  string
	State messages.ContainerState
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
	items       []SidebarItem
	groups      []string
	groupItems  map[string][]int
	collapsed   map[string]bool
	rows        []displayRow // flattened display rows
	cursor      int          // index into rows
	offset      int
	pendingG    bool // waiting for second 'g' in gg sequence
	searching   bool
	searchInput textinput.Model
	searchQuery string
	width       int
	height      int
	focused     bool
	yOffset     int // screen Y offset (e.g. tab bar height)
}

// NewSidebar creates a new sidebar.
func NewSidebar() Sidebar {
	ti := textinput.New()
	ti.Placeholder = "filter..."
	ti.CharLimit = 64
	return Sidebar{
		groupItems:  make(map[string][]int),
		collapsed:   make(map[string]bool),
		searchInput: ti,
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
			State: c.State,
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
	query := strings.ToLower(s.searchQuery)

	for _, group := range s.groups {
		indices := s.groupItems[group]
		if len(indices) == 0 {
			continue
		}

		// Filter items by search query.
		var filtered []int
		for _, idx := range indices {
			if query == "" || strings.Contains(strings.ToLower(s.items[idx].Name), query) {
				filtered = append(filtered, idx)
			}
		}
		if len(filtered) == 0 {
			continue
		}

		s.rows = append(s.rows, displayRow{isGroup: true, group: group})
		if !s.collapsed[group] {
			for _, idx := range filtered {
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
	if s.searching {
		return s.updateSearch(msg)
	}

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
			s.pendingG = false
			if s.cursor > 0 {
				s.cursor--
			}
		case key.Matches(msg, theme.Keys.Down):
			s.pendingG = false
			if s.cursor < len(s.rows)-1 {
				s.cursor++
			}
		case msg.String() == "G":
			s.cursor = len(s.rows) - 1
			s.pendingG = false
		case msg.String() == "g":
			if s.pendingG {
				s.cursor = 0
				s.pendingG = false
			} else {
				s.pendingG = true
			}
		case key.Matches(msg, theme.Keys.Search):
			s.pendingG = false
			s.searching = true
			s.searchInput.Focus()
			return nil
		case key.Matches(msg, theme.Keys.Enter):
			// Toggle collapse on group headers.
			s.pendingG = false
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

func (s *Sidebar) updateSearch(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc": //nolint:goconst // idiomatic key name
			s.searching = false
			s.searchQuery = ""
			s.searchInput.SetValue("")
			s.searchInput.Blur()
			s.rebuildRows()
			s.cursor = 0
			return nil
		case "enter": //nolint:goconst // idiomatic key name
			s.searchQuery = s.searchInput.Value()
			s.searching = false
			s.searchInput.Blur()
			s.rebuildRows()
			s.cursor = 0
			return nil
		}
	}

	var cmd tea.Cmd
	s.searchInput, cmd = s.searchInput.Update(msg)
	// Live filter as user types.
	s.searchQuery = s.searchInput.Value()
	s.rebuildRows()
	s.cursor = 0
	return cmd
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
	showSearchBar := s.searching || s.searchQuery != ""
	if showSearchBar {
		visibleHeight--
	}

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
			name := truncate(item.Name, s.width-6)

			if i == s.cursor && s.focused {
				// Selected: render plain text with full-row highlight (no separate icon styling).
				line := fmt.Sprintf("● %s", name)
				b.WriteString(theme.SidebarSelectedStyle.Width(s.width).Render(line))
			} else {
				icon := theme.StateIcon(int(item.State))
				line := fmt.Sprintf("%s %s", icon, name)
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

	// Search bar at bottom.
	if showSearchBar {
		if s.searching {
			b.WriteString(theme.SearchStyle.Render("/") + s.searchInput.View())
		} else {
			b.WriteString(theme.SearchStyle.Render(fmt.Sprintf("/%s", s.searchQuery)))
		}
		b.WriteString("\n")
	}

	sidebarStyle := theme.SidebarStyle
	if s.focused {
		sidebarStyle = sidebarStyle.BorderForeground(theme.SolBlue)
	}
	return sidebarStyle.Width(s.width).Height(s.height).Render(b.String())
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
