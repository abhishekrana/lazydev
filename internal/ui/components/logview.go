package components

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/abhishek-rana/lazydk/internal/ui/theme"
	"github.com/abhishek-rana/lazydk/pkg/messages"
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

	if !l.focused {
		return nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, theme.Keys.Up):
			l.autoScroll = false
			if l.offset > 0 {
				l.offset--
			}
		case key.Matches(msg, theme.Keys.Down):
			filtered := l.visibleLines()
			maxOffset := len(filtered) - l.viewableHeight()
			if maxOffset < 0 {
				maxOffset = 0
			}
			if l.offset < maxOffset {
				l.offset++
			}
			if l.offset >= maxOffset {
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
		case msg.String() == "G":
			l.pendingG = false
			l.autoScroll = true
			l.scrollToBottom()
		case msg.String() == "g":
			if l.pendingG {
				l.autoScroll = false
				l.offset = 0
				l.pendingG = false
			} else {
				l.pendingG = true
			}
		default:
			l.pendingG = false
		}
	}

	return nil
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
		header := theme.ActiveTabStyle.Width(l.width).Render(l.sourceLabel)
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

	lineCount := 0
	for i := start; i < end; i++ {
		rendered := l.renderLine(filtered[i])
		b.WriteString(rendered)
		b.WriteString("\n")
		lineCount++
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
	var parts []string

	// Source label for merged views.
	if line.Source != "" && l.sourceLabel == "" {
		sourceTag := fmt.Sprintf("[%s]", line.SourceID)
		parts = append(parts, theme.LogTimestampStyle.Render(sourceTag))
	}

	// Timestamp.
	if !line.Time.IsZero() {
		ts := line.Time.Format("15:04:05")
		parts = append(parts, theme.LogTimestampStyle.Render(ts))
	}

	// Text with level coloring and search highlighting.
	text := line.Text
	styled := l.styleText(text, line.Level)
	parts = append(parts, styled)

	result := strings.Join(parts, " ")
	if len(result) > l.width && l.width > 0 {
		return result[:l.width]
	}
	return fmt.Sprintf("%-*s", l.width, result)
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
