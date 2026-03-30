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

// LogView displays log lines with tail, search, and highlighting.
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
	viewable := l.viewableHeight()
	if len(l.lines) > viewable {
		l.offset = len(l.lines) - viewable
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
			maxOffset := len(l.lines) - l.viewableHeight()
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
		case msg.String() == "G":
			l.autoScroll = true
			l.scrollToBottom()
		case msg.String() == "g":
			l.autoScroll = false
			l.offset = 0
		}
	}

	return nil
}

func (l *LogView) updateSearch(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			l.searching = false
			l.searchQuery = ""
			l.searchInput.SetValue("")
			l.searchInput.Blur()
			return nil
		case "enter":
			l.searchQuery = l.searchInput.Value()
			l.searching = false
			l.searchInput.Blur()
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

	if l.searching {
		b.WriteString(theme.SearchStyle.Render("/") + l.searchInput.View())
	}

	return b.String()
}

func (l LogView) visibleLines() []messages.LogLine {
	if l.searchQuery == "" {
		return l.lines
	}

	var filtered []messages.LogLine
	query := strings.ToLower(l.searchQuery)
	for _, line := range l.lines {
		if strings.Contains(strings.ToLower(line.Text), query) {
			filtered = append(filtered, line)
		}
	}
	return filtered
}

func (l LogView) renderLine(line messages.LogLine) string {
	var parts []string

	if !line.Time.IsZero() {
		ts := line.Time.Format("15:04:05")
		parts = append(parts, theme.LogTimestampStyle.Render(ts))
	}

	text := line.Text
	var style lipgloss.Style
	switch line.Level {
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
		parts = append(parts, text)
		return fmt.Sprintf("%-*s", l.width, strings.Join(parts, " "))
	}

	parts = append(parts, style.Render(text))
	return fmt.Sprintf("%-*s", l.width, strings.Join(parts, " "))
}
