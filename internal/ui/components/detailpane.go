package components

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/abhishek-rana/lazydk/internal/ui/theme"
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
		header := theme.ActiveTabStyle.Width(d.width).Render(d.title)
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

	end := d.offset + viewable
	if end > len(d.lines) {
		end = len(d.lines)
	}

	lineCount := 0
	for i := d.offset; i < end; i++ {
		line := d.lines[i]
		if len(line) > d.width && d.width > 0 {
			line = line[:d.width]
		}
		b.WriteString(line)
		b.WriteString("\n")
		lineCount++
	}

	for lineCount < viewable {
		b.WriteString(strings.Repeat(" ", d.width))
		b.WriteString("\n")
		lineCount++
	}

	return b.String()
}
