package components

import (
	"os/exec"
	"regexp"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/abhishek-rana/lazydev/internal/ui/theme"
)

// urlPattern matches http/https URLs in plain text (after ANSI stripping).
var urlPattern = regexp.MustCompile(`https?://[^\s)\]>]+`)

// osc8Pattern captures the URL from an OSC 8 opener (ESC ] 8 ; ; URL
// followed by either ESC \ or BEL). Reference rows (#NNN / !NNN) wrap
// their IID in OSC 8; this is how the click handler resolves them
// since the URL itself never appears in the visible line text.
var osc8Pattern = regexp.MustCompile(`\x1b\]8;;([^\x1b\x07]+)(?:\x1b\\|\x07)`)

// urlOnLine returns the first URL on the line — OSC 8-wrapped if
// present, otherwise a plain http(s) URL in the visible text. Used by
// the Ctrl+click handler; matching is by row only (no column
// precision), which means clicks anywhere on a #NNN row open the
// linked item — short rows, intentional Ctrl modifier, acceptable.
func urlOnLine(line string) string {
	if m := osc8Pattern.FindStringSubmatch(line); len(m) > 1 {
		return m[1]
	}
	plain := ansiPattern.ReplaceAllString(line, "")
	return urlPattern.FindString(plain)
}

// ansiPattern matches ANSI escape sequences including OSC 8 hyperlinks.
// OSC sequences: \x1b]...(\x07|\x1b\\), CSI sequences: \x1b[...letter
var ansiPattern = regexp.MustCompile(`\x1b\].*?(?:\x07|\x1b\\)|\x1b\[[0-9;]*[a-zA-Z]`)

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
	yOffset  int // screen Y of the pane's top edge (e.g. tab bar height)
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

// Width returns the pane width.
func (d DetailPane) Width() int {
	return d.width
}

// SetFocused sets focus state.
func (d *DetailPane) SetFocused(focused bool) {
	d.focused = focused
}

// SetYOffset records the screen-Y row where the pane's top edge sits.
// Used by the mouse-click handler to translate screen-relative click
// coords into pane-local content rows.
func (d *DetailPane) SetYOffset(y int) {
	d.yOffset = y
}

// Focused returns focus state.
func (d DetailPane) Focused() bool {
	return d.focused
}

func (d DetailPane) viewableHeight() int {
	h := d.height
	if d.title != "" {
		// Title row + the blank spacer beneath it.
		h -= 2
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
		// Ctrl+click opens URL in browser.
		mouse := msg.Mouse()
		if !mouse.Mod.Contains(tea.ModCtrl) {
			return nil
		}
		// Translate screen-Y → pane-local content row:
		//   subtract the pane's screen offset (tab bar height),
		//   then subtract the in-pane chrome (title row + blank spacer).
		y := mouse.Y - d.yOffset
		if d.title != "" {
			y -= 2
		}
		lineIdx := d.offset + y
		if lineIdx < 0 || lineIdx >= len(d.lines) {
			return nil
		}
		if url := urlOnLine(d.lines[lineIdx]); url != "" {
			_ = exec.Command("xdg-open", url).Start() //nolint:gosec,noctx // intentional browser open
		}
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
		headerStyle := theme.DetailHeaderInactiveStyle
		if d.focused {
			headerStyle = theme.DetailHeaderActiveStyle
		}
		header := headerStyle.Width(d.width).Render(d.title)
		b.WriteString(header)
		b.WriteString("\n")
		// Spacer line between title and metadata strip.
		b.WriteString(strings.Repeat(" ", d.width))
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
		b.WriteString(d.lines[i])
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
