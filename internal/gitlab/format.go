package gitlab

import (
	"fmt"
	"strings"
	"time"

	"github.com/abhishek-rana/lazydev/pkg/messages"
)

// formatDateWithAge renders a timestamp as "2026-05-04 00:06 (9d ago)".
// Returns an empty string for zero times.
func formatDateWithAge(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	stamp := t.Format("2006-01-02 15:04")
	d := time.Since(t)
	var rel string
	switch {
	case d < 0:
		rel = "in the future"
	case d < time.Minute:
		rel = "just now"
	case d < time.Hour:
		rel = fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		rel = fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		rel = fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
	return fmt.Sprintf("%s  (%s)", stamp, rel)
}

// FormatIssueTitle is the single-line title rendered in the detail
// pane's header row above the metadata strip. State lives in the
// header strip below, not here — keeps the title focused on what the
// item is.
func FormatIssueTitle(issue messages.GitLabIssue) string {
	return fmt.Sprintf("#%d  %s", issue.IID, issue.Title)
}

// FormatMRTitle is the MR equivalent of FormatIssueTitle.
func FormatMRTitle(mr messages.GitLabMR) string {
	return fmt.Sprintf("!%d  %s", mr.IID, mr.Title)
}

// FormatState is the "<glyph> <state>" string used as the value of
// the State row in the header strip.
func FormatState(state string) string {
	return stateGlyph(state) + " " + state
}

const (
	// labelPad is the column width used to align metadata keys in the
	// header strip (e.g. "Assignees   …", "Iteration   …").
	labelPad = 12
	// narrowWidth is the threshold below which the header strip falls
	// back to "<key>: <value>" without padding, to stay readable on
	// 80-col-ish terminals.
	narrowWidth = 60
	// ruleWidth caps the horizontal-rule length so wide terminals don't
	// get unreadable dash bars.
	ruleWidth = 80
)

// labeled is one key/value row in the header strip. Rows whose value
// is empty are dropped by formatHeaderStrip rather than rendered as
// "None" — matches `gh issue view` style.
type labeled struct {
	k, v string
}

// emptyValue is rendered for any header-strip row whose value is
// empty — keeps the strip's vertical layout consistent across items
// regardless of which fields are populated.
const emptyValue = "—"

// formatHeaderStrip renders a block of aligned key/value lines. Empty
// values render as emptyValue rather than being dropped, so the strip
// has the same row layout across all items. When width is below
// narrowWidth, the alignment pad is dropped and rows render as
// "<key>: <value>".
func formatHeaderStrip(rows []labeled, width int) string {
	var b strings.Builder
	narrow := width > 0 && width < narrowWidth
	for _, r := range rows {
		v := r.v
		if v == "" {
			v = emptyValue
		}
		if narrow {
			b.WriteString(r.k)
			b.WriteString(": ")
			b.WriteString(v)
			b.WriteByte('\n')
			continue
		}
		b.WriteString(padRight(r.k, labelPad))
		b.WriteString(v)
		b.WriteByte('\n')
	}
	return b.String()
}

// rule returns a horizontal rule sized to width, capped at ruleWidth.
func rule(width int) string {
	w := width
	if w <= 0 || w > ruleWidth {
		w = ruleWidth
	}
	return strings.Repeat("─", w)
}

// commentSep is the thin divider between consecutive comments inside
// the Comments / Discussion block — lighter than `rule` so it reads as
// a subsection break rather than a major boundary.
func commentSep() string {
	return strings.Repeat("·", 40)
}

// stateGlyph returns a single-glyph indicator for issue / MR state.
// `opened` is the GitLab API spelling; we also accept `open` for
// caller convenience.
func stateGlyph(state string) string {
	switch state {
	case "opened", "open":
		return "●"
	case "closed":
		return "✗"
	case "merged":
		return "✓"
	case "locked":
		return "⊘"
	default:
		return "•"
	}
}

func padRight(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-len(s))
}
