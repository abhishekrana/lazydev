package components

import (
	"fmt"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/abhishek-rana/lazydev/internal/ui/theme"
)

// QueryLine is the single-line query DSL input shown above the sidebar.
//
// Lifecycle: hidden by default. Show() activates it; subsequent
// keypresses route through Update(). Esc dismisses and clears.
// Callers can read the current expression via Value() at any time.
type QueryLine struct {
	input    textinput.Model
	visible  bool
	prompt   string
	matchHit int    // last reported match count (0 hides the indicator)
	statusFG string // status hint text, e.g. error from cache
}

// NewQueryLine constructs an empty, hidden query line.
func NewQueryLine() QueryLine {
	ti := textinput.New()
	ti.Placeholder = "assignee:@me label:bug state:open  …"
	ti.CharLimit = 256
	return QueryLine{
		input:  ti,
		prompt: "/",
	}
}

// Show makes the query line visible and focuses its input.
func (q *QueryLine) Show() {
	q.visible = true
	q.input.Focus()
}

// Hide makes the query line invisible and blurs its input. The current
// value is preserved so the caller can re-show the same expression.
func (q *QueryLine) Hide() {
	q.visible = false
	q.input.Blur()
}

// Clear empties the input and hides.
func (q *QueryLine) Clear() {
	q.input.SetValue("")
	q.Hide()
}

// Visible returns whether the query line is on screen.
func (q QueryLine) Visible() bool { return q.visible }

// Value returns the current expression string (without the prompt).
func (q QueryLine) Value() string { return q.input.Value() }

// SetMatchCount displays an "N hits" indicator on the right edge.
// Set to zero to hide.
func (q *QueryLine) SetMatchCount(n int) { q.matchHit = n }

// SetStatus sets a small inline status hint (e.g., an error).
func (q *QueryLine) SetStatus(s string) { q.statusFG = s }

// Update processes a Bubble Tea message and returns a cmd. Returns
// (true, cmd) if the key event was an "Esc" that should bubble up to
// the caller as a close request — the caller is expected to call
// q.Hide() in that case.
//
// On Enter the query line stays open (typing more keys keeps live-
// filtering); callers can decide separately whether Enter should
// dismiss or commit to a saved view.
func (q *QueryLine) Update(msg tea.Msg) (escPressed bool, cmd tea.Cmd) {
	if !q.visible {
		return false, nil
	}
	if km, ok := msg.(tea.KeyPressMsg); ok {
		if km.String() == "esc" {
			return true, nil
		}
	}
	q.input, cmd = q.input.Update(msg)
	return false, cmd
}

// View renders the query line; empty string when hidden.
func (q QueryLine) View() string {
	if !q.visible && q.input.Value() == "" {
		return ""
	}
	prompt := theme.SearchStyle.Render(q.prompt)
	body := q.input.View()
	right := ""
	if q.matchHit > 0 {
		right = theme.LogTimestampStyle.Render(fmt.Sprintf("  %d hits", q.matchHit))
	} else if q.statusFG != "" {
		right = theme.StateErrorStyle.Render("  " + q.statusFG)
	}
	return prompt + body + right
}
