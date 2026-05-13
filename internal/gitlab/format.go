package gitlab

import (
	"fmt"
	"strings"
	"time"

	"github.com/abhishek-rana/lazydev/pkg/messages"
)

// linkify wraps text in an OSC 8 hyperlink (clickable in modern
// terminals) plus inline SGR codes that underline the text in
// Solarized blue, matching how glamour renders Markdown links in the
// body. Ghostty's OSC 8 hover-only underline isn't enough — we want
// links visually distinct at first paint.
//
// Returns the original text unchanged when url is empty.
//
// Sequence: ESC]8;;URL ESC\ ESC[4;38;2;38;139;210m TEXT ESC[0m ESC]8;;ESC\
func linkify(text, url string) string {
	if url == "" {
		return text
	}
	const (
		oscOpen  = "\x1b]8;;"
		oscMid   = "\x1b\\"
		sgrLink  = "\x1b[4;38;2;38;139;210m" // underline + SolBlue (#268BD2)
		sgrReset = "\x1b[0m"
		oscClose = "\x1b]8;;\x1b\\"
	)
	return oscOpen + url + oscMid + sgrLink + text + sgrReset + oscClose
}

// formatParent renders the Parent row value for the header strip.
// Returns "" when the issue has no parent so the strip helper renders
// the placeholder. siblingURL is any issue/work-item URL in the same
// project — we substitute the IID to derive the parent's URL, which
// avoids plumbing a separate ParentWebURL field through cache + GQL.
func formatParent(iid int64, title, siblingURL string) string {
	if iid == 0 && title == "" {
		return ""
	}
	var text string
	switch {
	case iid == 0:
		text = title
	case title == "":
		text = fmt.Sprintf("#%d", iid)
	default:
		text = fmt.Sprintf("#%d %s", iid, title)
	}
	return linkify(text, parentURL(siblingURL, iid))
}

// parentURL derives the parent's URL from a sibling's URL by swapping
// the trailing IID. Returns "" if siblingURL doesn't end in a numeric
// IID we can replace.
func parentURL(siblingURL string, parentIID int64) string {
	if siblingURL == "" || parentIID == 0 {
		return ""
	}
	idx := strings.LastIndex(siblingURL, "/")
	if idx < 0 || idx == len(siblingURL)-1 {
		return ""
	}
	tail := siblingURL[idx+1:]
	for _, r := range tail {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return siblingURL[:idx+1] + fmt.Sprintf("%d", parentIID)
}

// formatChildItems renders the "Child items (N)" footer block.
// Each child is `<glyph> #<iid> <title> [<type>]` aligned by glyph,
// wrapped in an OSC 8 hyperlink pointing at the child's WebURL.
func formatChildItems(items []messages.GitLabChildItem) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Child items (%d)\n", len(items))
	for _, c := range items {
		glyph := stateGlyph(c.State)
		typeSuffix := ""
		if c.ItemType != "" {
			typeSuffix = " [" + c.ItemType + "]"
		}
		row := fmt.Sprintf("%s #%d %s%s", glyph, c.IID, c.Title, typeSuffix)
		fmt.Fprintf(&b, "  %s\n", linkify(row, c.WebURL))
	}
	return b.String()
}

// linkedGroupOrder is the fixed display order for typed link groups —
// blockers first (most actionable), then what this item blocks, then
// loose relations.
var linkedGroupOrder = []struct {
	key, label string
}{
	{"is_blocked_by", "Blocked by"},
	{"blocks", "Blocks"},
	{"relates_to", "Relates to"},
}

// formatLinkedItems renders the "Linked items (N)" footer block,
// grouped by relation type in fixed order. Empty groups are skipped.
// Each link line is wrapped in an OSC 8 hyperlink to the linked item.
func formatLinkedItems(items []messages.GitLabLinkedItem) string {
	groups := make(map[string][]messages.GitLabLinkedItem)
	for _, it := range items {
		groups[it.LinkType] = append(groups[it.LinkType], it)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Linked items (%d)\n", len(items))
	for _, g := range linkedGroupOrder {
		set := groups[g.key]
		if len(set) == 0 {
			continue
		}
		fmt.Fprintf(&b, "  %s\n", g.label)
		for _, it := range set {
			glyph := stateGlyph(it.State)
			row := fmt.Sprintf("%s #%d %s  [%s]", glyph, it.IID, it.Title, it.State)
			fmt.Fprintf(&b, "    %s\n", linkify(row, it.WebURL))
		}
	}
	return b.String()
}

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
