package components

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/abhishek-rana/lazydk/internal/ui/theme"
	"github.com/abhishek-rana/lazydk/pkg/messages"
)

// Column defines a table column.
type Column struct {
	Title string
	Width int
	Key   string
}

// Table is a sortable resource table for the dashboard.
type Table struct {
	columns []Column
	rows    []messages.DashboardRow
	cursor  int
	offset  int
	sortCol int
	sortAsc bool
	width   int
	height  int
	focused bool
}

// NewTable creates a new table with dashboard columns.
func NewTable() Table {
	return Table{
		columns: []Column{
			{Title: "NAME", Width: 25, Key: "name"},
			{Title: "TYPE", Width: 10, Key: "type"},
			{Title: "SOURCE", Width: 10, Key: "source"},
			{Title: "GROUP", Width: 15, Key: "group"},
			{Title: "STATUS", Width: 15, Key: "status"},
			{Title: "RESTARTS", Width: 9, Key: "restarts"},
			{Title: "CPU", Width: 8, Key: "cpu"},
			{Title: "MEMORY", Width: 10, Key: "memory"},
		},
		sortAsc: true,
		focused: true,
	}
}

// SetRows updates the table data.
func (t *Table) SetRows(rows []messages.DashboardRow) {
	t.rows = rows
	t.sortRows()
	if t.cursor >= len(t.rows) {
		t.cursor = max(0, len(t.rows)-1)
	}
}

// SetSize sets table dimensions.
func (t *Table) SetSize(width, height int) {
	t.width = width
	t.height = height
	t.distributeWidths()
}

// SetFocused sets focus state.
func (t *Table) SetFocused(focused bool) {
	t.focused = focused
}

// SelectedRow returns the currently selected row.
func (t Table) SelectedRow() (messages.DashboardRow, bool) {
	if len(t.rows) == 0 || t.cursor < 0 || t.cursor >= len(t.rows) {
		return messages.DashboardRow{}, false
	}
	return t.rows[t.cursor], true
}

func (t *Table) distributeWidths() {
	totalFixed := 0
	for _, c := range t.columns {
		totalFixed += c.Width
	}
	// If there's extra space, give it to the Name column.
	extra := t.width - totalFixed - len(t.columns) - 1 // account for separators
	if extra > 0 {
		t.columns[0].Width += extra
	}
}

// Update handles input.
func (t *Table) Update(msg tea.Msg) tea.Cmd {
	if !t.focused {
		return nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, theme.Keys.Up):
			if t.cursor > 0 {
				t.cursor--
			}
		case key.Matches(msg, theme.Keys.Down):
			if t.cursor < len(t.rows)-1 {
				t.cursor++
			}
		case msg.String() == "1":
			t.toggleSort(0)
		case msg.String() == "2":
			t.toggleSort(1)
		case msg.String() == "3":
			t.toggleSort(2)
		case msg.String() == "4":
			t.toggleSort(3)
		case msg.String() == "5":
			t.toggleSort(4)
		case msg.String() == "6":
			t.toggleSort(5)
		}
	}

	return nil
}

func (t *Table) toggleSort(col int) {
	if t.sortCol == col {
		t.sortAsc = !t.sortAsc
	} else {
		t.sortCol = col
		t.sortAsc = true
	}
	t.sortRows()
}

func (t *Table) sortRows() {
	col := t.sortCol
	asc := t.sortAsc

	sort.SliceStable(t.rows, func(i, j int) bool {
		a := t.cellValue(t.rows[i], col)
		b := t.cellValue(t.rows[j], col)
		if asc {
			return a < b
		}
		return a > b
	})
}

func (t Table) cellValue(row messages.DashboardRow, col int) string {
	switch col {
	case 0:
		return row.Name
	case 1:
		return row.Type
	case 2:
		return row.Source
	case 3:
		return row.Group
	case 4:
		return row.Status
	case 5:
		return fmt.Sprintf("%05d", row.Restarts)
	default:
		return ""
	}
}

// View renders the table.
func (t Table) View() string {
	var b strings.Builder

	// Header row.
	b.WriteString(t.renderHeader())
	b.WriteString("\n")

	// Separator.
	b.WriteString(theme.LogTimestampStyle.Render(strings.Repeat("─", t.width)))
	b.WriteString("\n")

	// Visible rows.
	bodyHeight := t.height - 3 // header + separator + status line
	if bodyHeight < 1 {
		bodyHeight = 1
	}

	if t.cursor < t.offset {
		t.offset = t.cursor
	}
	if t.cursor >= t.offset+bodyHeight {
		t.offset = t.cursor - bodyHeight + 1
	}

	end := t.offset + bodyHeight
	if end > len(t.rows) {
		end = len(t.rows)
	}

	lineCount := 0
	for i := t.offset; i < end; i++ {
		row := t.rows[i]
		rendered := t.renderRow(row, i == t.cursor)
		b.WriteString(rendered)
		b.WriteString("\n")
		lineCount++
	}

	for lineCount < bodyHeight {
		b.WriteString(strings.Repeat(" ", t.width))
		b.WriteString("\n")
		lineCount++
	}

	// Status line.
	status := fmt.Sprintf(" %d resources  [1-6] sort  [j/k] navigate", len(t.rows))
	b.WriteString(theme.StatusBarStyle.Width(t.width).Render(status))

	return b.String()
}

func (t Table) renderHeader() string {
	cells := make([]string, 0, len(t.columns))
	for i, col := range t.columns {
		title := col.Title
		if i == t.sortCol {
			arrow := "▲"
			if !t.sortAsc {
				arrow = "▼"
			}
			title = fmt.Sprintf("%s %s", title, arrow)
		}
		cells = append(cells, theme.SidebarGroupStyle.Render(padRight(title, col.Width)))
	}
	return strings.Join(cells, " ")
}

func (t Table) renderRow(row messages.DashboardRow, selected bool) string {
	stateIcon := theme.StateIcon(int(row.State))
	cells := []string{
		padRight(row.Name, t.columns[0].Width),
		padRight(row.Type, t.columns[1].Width),
		padRight(row.Source, t.columns[2].Width),
		padRight(row.Group, t.columns[3].Width),
		padRight(stateIcon+" "+row.Status, t.columns[4].Width),
		padRight(fmt.Sprintf("%d", row.Restarts), t.columns[5].Width),
		padRight(row.CPU, t.columns[6].Width),
		padRight(row.Memory, t.columns[7].Width),
	}

	line := strings.Join(cells, " ")

	if selected && t.focused {
		return theme.SidebarSelectedStyle.Width(t.width).Render(line)
	}
	return theme.SidebarItemStyle.Width(t.width).Render(line)
}

func padRight(s string, width int) string {
	if len(s) >= width {
		if width > 3 {
			return s[:width-3] + "..."
		}
		return s[:width]
	}
	return s + strings.Repeat(" ", width-len(s))
}
