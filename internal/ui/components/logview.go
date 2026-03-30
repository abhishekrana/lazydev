package components

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/abhishek-rana/lazydev/internal/export"
	"github.com/abhishek-rana/lazydev/internal/ui/theme"
	"github.com/abhishek-rana/lazydev/pkg/messages"
)

// LogView displays log lines with tail, search, level filtering, and highlighting.
type LogView struct {
	lines       []messages.LogLine
	offset      int
	width       int
	height      int
	focused     bool
	autoScroll  bool
	searching   bool
	searchInput textinput.Model
	searchQuery string
	sourceLabel string
	levelFilter messages.LogLevel // LogLevelUnknown = show all
	pendingG    bool
	wrapLines   bool
	cursor      int // current line index in filtered lines
	yOffset     int // screen Y offset for mouse coordinate mapping
	xOffset     int // screen X offset for mouse coordinate mapping
}

// NewLogView creates a new log viewport.
func NewLogView() LogView {
	ti := textinput.New()
	ti.Placeholder = "search..."
	ti.CharLimit = 256

	return LogView{
		autoScroll:  true,
		searchInput: ti,
	}
}

// SetSize sets the logview dimensions.
func (l *LogView) SetSize(width, height int) {
	l.width = width
	l.height = height
}

// SetOffset sets the screen offset for mouse coordinate mapping.
func (l *LogView) SetOffset(x, y int) {
	l.xOffset = x
	l.yOffset = y
}

// SetFocused sets whether the logview has focus.
func (l *LogView) SetFocused(focused bool) {
	l.focused = focused
}

// Focused returns whether the logview has focus.
func (l LogView) Focused() bool {
	return l.focused
}

// SetSourceLabel sets the label shown at the top.
func (l *LogView) SetSourceLabel(label string) {
	l.sourceLabel = label
}

// AppendLines adds log lines.
func (l *LogView) AppendLines(lines []messages.LogLine) {
	l.lines = append(l.lines, lines...)

	maxLines := 10000
	if len(l.lines) > maxLines {
		l.lines = l.lines[len(l.lines)-maxLines:]
	}

	if l.autoScroll {
		l.scrollToBottom()
	}
}

// Clear removes all log lines.
func (l *LogView) Clear() {
	l.lines = nil
	l.offset = 0
}

func (l *LogView) scrollToBottom() {
	filtered := l.visibleLines()
	viewable := l.viewableHeight()
	l.cursor = max(0, len(filtered)-1)
	if len(filtered) > viewable {
		l.offset = len(filtered) - viewable
	} else {
		l.offset = 0
	}
}

func (l LogView) viewableHeight() int {
	h := l.height
	if l.sourceLabel != "" {
		h--
	}
	if l.searching {
		h--
	}
	// Status line (filter/search info).
	h--
	if h < 1 {
		h = 1
	}
	return h
}

// Update handles input.
func (l *LogView) Update(msg tea.Msg) tea.Cmd {
	if l.searching {
		return l.updateSearch(msg)
	}

	// Handle mouse events even when not focused (tabs forward clicks).
	switch msg := msg.(type) {
	case tea.MouseClickMsg:
		mouse := msg.Mouse()
		row := mouse.Y - l.yOffset
		// Account for header line.
		if l.sourceLabel != "" {
			row--
		}
		if row >= 0 && row < l.viewableHeight() {
			filtered := l.visibleLines()
			idx := l.offset + row
			if idx >= 0 && idx < len(filtered) {
				l.cursor = idx
				l.autoScroll = false
				if idx >= len(filtered)-1 {
					l.autoScroll = true
				}
			}
		}
		return nil

	case tea.MouseWheelMsg:
		mouse := msg.Mouse()
		filtered := l.visibleLines()
		viewable := l.viewableHeight()
		switch mouse.Button {
		case tea.MouseWheelUp:
			l.autoScroll = false
			l.offset -= 3
			if l.offset < 0 {
				l.offset = 0
			}
			// Keep cursor in visible range.
			if l.cursor >= l.offset+viewable {
				l.cursor = l.offset + viewable - 1
			}
			if l.cursor < l.offset {
				l.cursor = l.offset
			}
		case tea.MouseWheelDown:
			l.offset += 3
			maxOffset := len(filtered) - viewable
			if maxOffset < 0 {
				maxOffset = 0
			}
			if l.offset > maxOffset {
				l.offset = maxOffset
			}
			// Keep cursor in visible range.
			if l.cursor < l.offset {
				l.cursor = l.offset
			}
			if l.cursor >= l.offset+viewable && len(filtered) > 0 {
				l.cursor = l.offset + viewable - 1
			}
			if l.cursor >= len(filtered)-1 {
				l.autoScroll = true
			}
		}
		return nil
	}

	if !l.focused {
		return nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, theme.Keys.Up):
			l.autoScroll = false
			if l.cursor > 0 {
				l.cursor--
			}
			// Scroll viewport to keep cursor visible.
			if l.cursor < l.offset {
				l.offset = l.cursor
			}
		case key.Matches(msg, theme.Keys.Down):
			filtered := l.visibleLines()
			if l.cursor < len(filtered)-1 {
				l.cursor++
			}
			// Scroll viewport to keep cursor visible.
			viewable := l.viewableHeight()
			if l.cursor >= l.offset+viewable {
				l.offset = l.cursor - viewable + 1
			}
			if l.cursor >= len(filtered)-1 {
				l.autoScroll = true
			}
		case key.Matches(msg, theme.Keys.Search):
			l.searching = true
			l.searchInput.Focus()
			return nil
		case key.Matches(msg, theme.Keys.Filter):
			l.cycleFilter()
			if l.autoScroll {
				l.scrollToBottom()
			}
		case msg.String() == "w":
			l.wrapLines = !l.wrapLines
			l.offset = 0
			if l.autoScroll {
				l.scrollToBottom()
			}
		case msg.String() == "G":
			l.pendingG = false
			l.autoScroll = true
			filtered := l.visibleLines()
			l.cursor = max(0, len(filtered)-1)
			l.scrollToBottom()
		case msg.String() == "g":
			if l.pendingG {
				l.autoScroll = false
				l.offset = 0
				l.cursor = 0
				l.pendingG = false
			} else {
				l.pendingG = true
			}
		case msg.String() == "y":
			// Yank cursor line to clipboard.
			l.pendingG = false
			filtered := l.visibleLines()
			if l.cursor >= 0 && l.cursor < len(filtered) {
				text := export.LinesToText(filtered[l.cursor : l.cursor+1])
				return tea.Printf("%s", export.ToClipboardOSC52(text))
			}
		case msg.String() == "Y":
			// Yank all visible/filtered lines to clipboard.
			l.pendingG = false
			filtered := l.visibleLines()
			if len(filtered) > 0 {
				text := export.LinesToText(filtered)
				return tea.Printf("%s", export.ToClipboardOSC52(text))
			}
		case msg.String() == "e":
			// Export visible logs to text file.
			l.pendingG = false
			return l.exportToFile(false)
		case msg.String() == "E":
			// Export visible logs to JSON file.
			l.pendingG = false
			return l.exportToFile(true)
		case msg.String() == "o":
			// Open filtered logs in $EDITOR.
			l.pendingG = false
			return l.openInEditor()
		default:
			l.pendingG = false
		}
	}

	return nil
}

func (l *LogView) exportToFile(asJSON bool) tea.Cmd {
	filtered := l.visibleLines()
	label := l.sourceLabel
	if label == "" {
		label = "all-logs"
	}

	return func() tea.Msg {
		var content, ext string
		if asJSON {
			content = export.LinesToJSON(filtered)
			ext = ".json"
		} else {
			content = export.LinesToText(filtered)
			ext = ".log"
		}

		path, err := export.ToFile(label, content, ext)
		return messages.LogExportedMsg{Path: path, Err: err}
	}
}

func (l *LogView) openInEditor() tea.Cmd {
	filtered := l.visibleLines()
	label := l.sourceLabel
	if label == "" {
		label = "all-logs"
	}

	content := export.LinesToText(filtered)
	path, err := export.ToFile(label, content, ".log")
	if err != nil {
		return func() tea.Msg {
			return messages.LogExportedMsg{Err: err}
		}
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}

	// Open at cursor line position.
	lineArg := fmt.Sprintf("+%d", l.cursor+1)
	c := exec.Command(editor, lineArg, path) //nolint:gosec,noctx // intentional editor open, ExecProcess manages lifecycle
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return messages.ExecFinishedMsg{Err: err}
	})
}

func (l *LogView) cycleFilter() {
	switch l.levelFilter {
	case messages.LogLevelUnknown:
		l.levelFilter = messages.LogLevelError
	case messages.LogLevelError:
		l.levelFilter = messages.LogLevelWarn
	case messages.LogLevelWarn:
		l.levelFilter = messages.LogLevelInfo
	case messages.LogLevelInfo:
		l.levelFilter = messages.LogLevelDebug
	default:
		l.levelFilter = messages.LogLevelUnknown
	}
	l.offset = 0
}

func (l *LogView) updateSearch(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc": //nolint:goconst // idiomatic key name
			l.searching = false
			l.searchQuery = ""
			l.searchInput.SetValue("")
			l.searchInput.Blur()
			return nil
		case "enter": //nolint:goconst // idiomatic key name
			l.searchQuery = l.searchInput.Value()
			l.searching = false
			l.searchInput.Blur()
			if l.autoScroll {
				l.scrollToBottom()
			}
			return nil
		}
	}

	var cmd tea.Cmd
	l.searchInput, cmd = l.searchInput.Update(msg)
	return cmd
}

// View renders the log view.
func (l LogView) View() string {
	var b strings.Builder

	if l.sourceLabel != "" {
		headerStyle := theme.InactiveHeaderStyle
		if l.focused {
			headerStyle = theme.ActiveTabStyle
		}
		header := headerStyle.Width(l.width).Render(l.sourceLabel)
		b.WriteString(header)
		b.WriteString("\n")
	}

	viewable := l.viewableHeight()
	filtered := l.visibleLines()

	start := l.offset
	if start < 0 {
		start = 0
	}
	end := start + viewable
	if end > len(filtered) {
		end = len(filtered)
	}

	cursorStyle := theme.LogCursorStyle

	lineCount := 0
	for i := start; i < end && lineCount < viewable; i++ {
		isCursor := (i == l.cursor) && l.focused

		if l.wrapLines && l.width > 0 {
			raw := l.rawLine(filtered[i])
			firstChunk := true
			for len(raw) > 0 && lineCount < viewable {
				chunk := raw
				if len(chunk) > l.width {
					chunk = raw[:l.width]
					raw = raw[l.width:]
				} else {
					raw = ""
				}
				if isCursor && firstChunk {
					b.WriteString(cursorStyle.Width(l.width).Render(chunk))
					firstChunk = false
				} else {
					styled := l.styleText(chunk, filtered[i].Level)
					b.WriteString(styled)
				}
				b.WriteString("\n")
				lineCount++
			}
		} else {
			if isCursor {
				b.WriteString(cursorStyle.Width(l.width).Render(l.rawLineTruncated(filtered[i])))
			} else {
				b.WriteString(l.renderLine(filtered[i]))
			}
			b.WriteString("\n")
			lineCount++
		}
	}

	for lineCount < viewable {
		b.WriteString(strings.Repeat(" ", l.width))
		b.WriteString("\n")
		lineCount++
	}

	// Status line.
	b.WriteString(l.renderStatusLine(len(filtered)))

	// Search bar.
	if l.searching {
		b.WriteString("\n")
		b.WriteString(theme.SearchStyle.Render("/") + l.searchInput.View())
	}

	return b.String()
}

func (l LogView) renderStatusLine(filteredCount int) string {
	var parts []string

	// Level filter indicator.
	levelName := levelFilterName(l.levelFilter)
	if l.levelFilter != messages.LogLevelUnknown {
		parts = append(parts, theme.SearchStyle.Render(fmt.Sprintf("[f]ilter:%s", levelName)))
	} else {
		parts = append(parts, theme.LogTimestampStyle.Render("[f]ilter:ALL"))
	}

	// Wrap indicator.
	if l.wrapLines {
		parts = append(parts, theme.SearchStyle.Render("[w]rap:ON"))
	} else {
		parts = append(parts, theme.LogTimestampStyle.Render("[w]rap:OFF"))
	}

	// Search indicator.
	if l.searchQuery != "" {
		parts = append(parts, theme.SearchStyle.Render(fmt.Sprintf("[/]search:%q", l.searchQuery)))
	}

	// Line count.
	total := len(l.lines)
	if filteredCount != total {
		parts = append(parts, theme.LogTimestampStyle.Render(fmt.Sprintf("%d/%d lines", filteredCount, total)))
	} else {
		parts = append(parts, theme.LogTimestampStyle.Render(fmt.Sprintf("%d lines", total)))
	}

	content := strings.Join(parts, "  ")
	return theme.StatusBarStyle.Width(l.width).Render(content)
}

func levelFilterName(level messages.LogLevel) string {
	switch level {
	case messages.LogLevelError:
		return "ERROR+"
	case messages.LogLevelWarn:
		return "WARN+"
	case messages.LogLevelInfo:
		return "INFO+"
	case messages.LogLevelDebug:
		return "DEBUG+"
	default:
		return "ALL"
	}
}

// rawLineTruncated returns unstyled text truncated to pane width (for cursor highlight).
func (l LogView) rawLineTruncated(line messages.LogLine) string {
	raw := l.rawLine(line)
	if l.width > 0 && len(raw) > l.width {
		return raw[:l.width]
	}
	return raw
}

// rawLine returns the unstyled text for a log line (for wrapping).
func (l LogView) rawLine(line messages.LogLine) string {
	var parts []string
	if line.Source != "" && l.sourceLabel == "" {
		parts = append(parts, fmt.Sprintf("[%s]", line.SourceID))
	}
	if !line.Time.IsZero() {
		parts = append(parts, line.Time.Format("15:04:05"))
	}
	parts = append(parts, line.Text)
	return strings.Join(parts, " ")
}

func (l LogView) visibleLines() []messages.LogLine {
	lines := l.lines

	// Apply level filter.
	if l.levelFilter != messages.LogLevelUnknown {
		var filtered []messages.LogLine
		for _, line := range lines {
			if line.Level >= l.levelFilter {
				filtered = append(filtered, line)
			}
		}
		lines = filtered
	}

	// Apply search filter.
	if l.searchQuery != "" {
		var filtered []messages.LogLine
		query := strings.ToLower(l.searchQuery)
		for _, line := range lines {
			if strings.Contains(strings.ToLower(line.Text), query) {
				filtered = append(filtered, line)
			}
		}
		lines = filtered
	}

	return lines
}

func (l LogView) renderLine(line messages.LogLine) string {
	var prefixParts []string
	prefixLen := 0

	// Source label for merged views.
	if line.Source != "" && l.sourceLabel == "" {
		tag := fmt.Sprintf("[%s]", line.SourceID)
		prefixParts = append(prefixParts, theme.LogTimestampStyle.Render(tag))
		prefixLen += len(tag) + 1
	}

	// Timestamp.
	if !line.Time.IsZero() {
		ts := line.Time.Format("15:04:05")
		prefixParts = append(prefixParts, theme.LogTimestampStyle.Render(ts))
		prefixLen += len(ts) + 1
	}

	// Truncate raw text to fit width (accounting for prefix).
	text := line.Text
	if l.width > 0 {
		maxText := l.width - prefixLen
		if maxText < 10 {
			maxText = 10
		}
		if len(text) > maxText {
			text = text[:maxText]
		}
	}

	styled := l.styleText(text, line.Level)
	prefixParts = append(prefixParts, styled)

	return strings.Join(prefixParts, " ")
}

func (l LogView) styleText(text string, level messages.LogLevel) string {
	// Apply search highlighting.
	if l.searchQuery != "" {
		text = l.highlightSearch(text)
	}

	// Apply level coloring.
	var style lipgloss.Style
	switch level {
	case messages.LogLevelDebug:
		style = theme.LogDebugStyle
	case messages.LogLevelInfo:
		style = theme.LogInfoStyle
	case messages.LogLevelWarn:
		style = theme.LogWarnStyle
	case messages.LogLevelError:
		style = theme.LogErrorStyle
	case messages.LogLevelFatal:
		style = theme.LogFatalStyle
	default:
		return text
	}

	return style.Render(text)
}

func (l LogView) highlightSearch(text string) string {
	if l.searchQuery == "" {
		return text
	}

	lower := strings.ToLower(text)
	query := strings.ToLower(l.searchQuery)
	idx := strings.Index(lower, query)
	if idx < 0 {
		return text
	}

	// Highlight the first match.
	before := text[:idx]
	match := text[idx : idx+len(l.searchQuery)]
	after := text[idx+len(l.searchQuery):]

	highlighted := theme.SearchStyle.Bold(true).Render(match)
	return before + highlighted + after
}
