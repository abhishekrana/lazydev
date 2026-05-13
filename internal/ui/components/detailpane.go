package components

import (
	"os/exec"
	"regexp"
	"strings"
	"unicode/utf8"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/abhishek-rana/lazydev/internal/ui/theme"
)

// urlPattern matches http/https URLs in plain text (after ANSI stripping).
var urlPattern = regexp.MustCompile(`https?://[^\s)\]>]+`)

// urlAtColumn returns the URL the user clicked on at pane-local column
// col on the given line, or "" if the click missed every link.
//
// Resolution order:
//  1. Scan the line for OSC 8 link spans, tracking visible column
//     position as escapes are passed over. If col falls inside any
//     span, return that span's URL.
//  2. Fall back to a plain http(s) URL whose substring contains col
//     (after stripping ANSI). Catches the URL row and Markdown bodies
//     where the URL is also the visible text.
func urlAtColumn(line string, col int) string {
	if col < 0 {
		return ""
	}
	if url := osc8URLAtColumn(line, col); url != "" {
		return url
	}
	return plainURLAtColumn(line, col)
}

// osc8URLAtColumn walks the line tracking visible-column position as
// it skips over ANSI escapes; whenever it sees an OSC 8 open it
// records the start column, and on the matching empty-URL OSC 8 close
// it checks whether col falls inside [start, end).
func osc8URLAtColumn(line string, col int) string {
	visCol := 0
	i := 0
	type span struct {
		url      string
		startCol int
		open     bool
	}
	var cur span
	for i < len(line) {
		// OSC 8 open: ESC ] 8 ; ; <URL> (ESC \\ | BEL)
		if i+3 < len(line) && line[i] == 0x1b && line[i+1] == ']' && line[i+2] == '8' && line[i+3] == ';' {
			// Find terminator.
			term := -1
			termLen := 0
			for j := i + 4; j < len(line); j++ {
				if line[j] == 0x07 {
					term, termLen = j, 1
					break
				}
				if line[j] == 0x1b && j+1 < len(line) && line[j+1] == '\\' {
					term, termLen = j, 2
					break
				}
			}
			if term < 0 {
				return ""
			}
			// Body is between i+4 and term, formatted as ";<URL>".
			body := line[i+4 : term]
			body = strings.TrimPrefix(body, ";")
			if body == "" {
				// Close: did we land inside the open span?
				if cur.open && col >= cur.startCol && col < visCol {
					return cur.url
				}
				cur = span{}
			} else {
				cur = span{url: body, startCol: visCol, open: true}
			}
			i = term + termLen
			continue
		}
		// CSI escape: ESC [ ... letter — skip without advancing visCol.
		if i+1 < len(line) && line[i] == 0x1b && line[i+1] == '[' {
			j := i + 2
			for j < len(line) {
				c := line[j]
				if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
					j++
					break
				}
				j++
			}
			i = j
			continue
		}
		// Visible rune — advance one column. Multi-byte UTF-8 runes
		// take one column for the typical issue title characters we
		// surface; double-wide CJK would need lipgloss.Width here.
		_, size := utf8.DecodeRuneInString(line[i:])
		if size == 0 {
			size = 1
		}
		visCol++
		i += size
	}
	return ""
}

// plainURLAtColumn finds an http(s) URL whose visible column range
// contains col. Used for the URL row and Markdown body where the URL
// is the rendered text.
func plainURLAtColumn(line string, col int) string {
	plain := ansiPattern.ReplaceAllString(line, "")
	matches := urlPattern.FindAllStringIndex(plain, -1)
	for _, m := range matches {
		// m is byte-offset; column is rune-offset. For ASCII URLs the
		// two coincide; for surrounding non-ASCII text they may
		// diverge, but URLs in our content are always ASCII.
		startCol := utf8.RuneCountInString(plain[:m[0]])
		endCol := utf8.RuneCountInString(plain[:m[1]])
		if col >= startCol && col < endCol {
			return plain[m[0]:m[1]]
		}
	}
	return ""
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
	xOffset  int // screen X of the pane's left edge (e.g. sidebar width)
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

// SetXOffset records the screen-X column where the pane's left edge
// sits (i.e. the sidebar width). Lets Ctrl+click resolve which
// OSC 8 span the cursor is over instead of just any link on the row.
func (d *DetailPane) SetXOffset(x int) {
	d.xOffset = x
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
		localX := mouse.X - d.xOffset
		url := urlAtColumn(d.lines[lineIdx], localX)
		if url != "" {
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
