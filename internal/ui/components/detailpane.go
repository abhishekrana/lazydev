package components

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/abhishek-rana/lazydev/internal/ui/theme"
)

// DetailPane displays formatted text (JSON inspect, YAML, describe output).
type DetailPane struct {
	content  string
	title    string
	offset   int
	width    int
	height   int
	focused  bool
	lines    []string
	pendingG bool
}

// NewDetailPane creates a new detail pane.
func NewDetailPane() DetailPane {
	return DetailPane{}
}

// SetContent sets the text content to display.
func (d *DetailPane) SetContent(title, content string) {
	d.title = title
	d.content = content
	d.lines = strings.Split(content, "\n")
	d.offset = 0
}

// Clear empties the detail pane.
func (d *DetailPane) Clear() {
	d.content = ""
	d.title = ""
	d.lines = nil
	d.offset = 0
}

// SetSize sets dimensions.
func (d *DetailPane) SetSize(width, height int) {
	d.width = width
	d.height = height
}

// SetFocused sets focus state.
func (d *DetailPane) SetFocused(focused bool) {
	d.focused = focused
}

// Focused returns focus state.
func (d DetailPane) Focused() bool {
	return d.focused
}

func (d DetailPane) viewableHeight() int {
	h := d.height
	if d.title != "" {
		h--
	}
	if h < 1 {
		h = 1
	}
	return h
}

// Update handles scrolling input.
func (d *DetailPane) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.MouseWheelMsg:
		mouse := msg.Mouse()
		viewable := d.viewableHeight()
		maxOffset := len(d.lines) - viewable
		if maxOffset < 0 {
			maxOffset = 0
		}
		switch mouse.Button {
		case tea.MouseWheelUp:
			d.offset -= 3
			if d.offset < 0 {
				d.offset = 0
			}
		case tea.MouseWheelDown:
			d.offset += 3
			if d.offset > maxOffset {
				d.offset = maxOffset
			}
		}
		return nil

	case tea.MouseClickMsg:
		return nil
	}

	if !d.focused {
		return nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, theme.Keys.Up):
			if d.offset > 0 {
				d.offset--
			}
		case key.Matches(msg, theme.Keys.Down):
			maxOffset := len(d.lines) - d.viewableHeight()
			if maxOffset < 0 {
				maxOffset = 0
			}
			if d.offset < maxOffset {
				d.offset++
			}
		case msg.String() == "G":
			d.pendingG = false
			maxOffset := len(d.lines) - d.viewableHeight()
			if maxOffset > 0 {
				d.offset = maxOffset
			}
		case msg.String() == "g":
			if d.pendingG {
				d.offset = 0
				d.pendingG = false
			} else {
				d.pendingG = true
			}
		default:
			d.pendingG = false
		}
	}

	return nil
}

// View renders the detail pane.
func (d DetailPane) View() string {
	var b strings.Builder

	if d.title != "" {
		headerStyle := theme.InactiveHeaderStyle
		if d.focused {
			headerStyle = theme.ActiveTabStyle
		}
		header := headerStyle.Width(d.width).Render(d.title)
		b.WriteString(header)
		b.WriteString("\n")
	}

	viewable := d.viewableHeight()

	if len(d.lines) == 0 {
		b.WriteString(theme.SidebarItemStyle.Render("  No data"))
		b.WriteString("\n")
		for i := 1; i < viewable; i++ {
			b.WriteString(strings.Repeat(" ", d.width))
			b.WriteString("\n")
		}
		return b.String()
	}

	contentWidth := d.width - 1 // reserve 1 col for scrollbar
	if contentWidth < 1 {
		contentWidth = 1
	}

	end := d.offset + viewable
	if end > len(d.lines) {
		end = len(d.lines)
	}

	thumbStart, thumbEnd := d.scrollbarThumb(len(d.lines), viewable, d.offset)

	lineCount := 0
	for i := d.offset; i < end; i++ {
		line := d.lines[i]
		if len(line) > contentWidth && contentWidth > 0 {
			line = line[:contentWidth]
		}
		b.WriteString(line)
		// Pad to contentWidth for scrollbar alignment.
		if pad := contentWidth - len(line); pad > 0 {
			b.WriteString(strings.Repeat(" ", pad))
		}
		b.WriteString(d.scrollbarChar(lineCount, thumbStart, thumbEnd))
		b.WriteString("\n")
		lineCount++
	}

	for lineCount < viewable {
		b.WriteString(strings.Repeat(" ", contentWidth))
		b.WriteString(d.scrollbarChar(lineCount, thumbStart, thumbEnd))
		b.WriteString("\n")
		lineCount++
	}

	return b.String()
}

func (d DetailPane) scrollbarThumb(totalLines, viewable, offset int) (int, int) {
	if totalLines <= viewable || viewable <= 0 {
		return -1, -1
	}
	thumbSize := max(1, viewable*viewable/totalLines)
	thumbPos := offset * (viewable - thumbSize) / max(1, totalLines-viewable)
	return thumbPos, thumbPos + thumbSize
}

func (d DetailPane) scrollbarChar(row, thumbStart, thumbEnd int) string {
	if thumbStart < 0 {
		return " "
	}
	if row >= thumbStart && row < thumbEnd {
		return theme.ScrollbarThumbStyle.Render("┃")
	}
	return " "
}
