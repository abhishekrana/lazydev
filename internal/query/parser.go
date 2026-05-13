// Package query parses the lazydev query DSL — a k9s/GitLab-style
// expression mixing `key:value` operators with bare fuzzy terms.
//
// Examples:
//
//	assignee:@me label:bug state:open    // mine, bug-tagged, open
//	assignee:@ai                         // Claude's queue
//	updated:>7d kind:mr                  // active MRs this week
//	refresh                              // fuzzy across title/body/notes
//
// Tokens are whitespace-separated and AND'd together. `|` between two
// values selects either match for the same field. `@me` and `@ai`
// resolve to the authenticated user and `cfg.GitLab.AIUser`
// respectively.
package query

import (
	"strings"
	"time"

	"github.com/abhishek-rana/lazydev/internal/cache"
)

// Env carries user-context variables referenced by `@`-tokens.
type Env struct {
	// Me is the authenticated GitLab user's username (no `@` prefix).
	Me string
	// AI is the configured AI-handoff user (cfg.GitLab.AIUser).
	AI string
	// Now is the reference time for `updated:` ranges. Defaults to
	// time.Now() when zero.
	Now time.Time
}

// Expression is the parsed form of a query string. Kind tells callers
// which collection(s) to search (issues, mrs, or both). Filter holds
// the structured constraints to forward to cache.Store.
type Expression struct {
	// Kind is "issue", "mr", or "" (both / unspecified).
	Kind string
	// Filter is the structured filter passed to cache.ListIssues /
	// cache.ListMRs. Bare fuzzy terms land in Filter.Text.
	Filter cache.Filter
	// UpdatedAfter, when non-zero, narrows results to updated_at > t.
	// Filled from `updated:>7d` / `updated:<30d` tokens. List* in the
	// cache does not yet honor this; Apply() returns it for callers
	// that want to post-filter in Go.
	UpdatedAfter time.Time
	// UpdatedBefore is the other side of an `updated:<…` range.
	UpdatedBefore time.Time
}

// Parse turns a query string into an Expression. It never returns an
// error: unrecognized tokens are passed through as bare fuzzy terms,
// so users get partial matches instead of "syntax error" footguns.
func Parse(input string, env Env) Expression {
	if env.Now.IsZero() {
		env.Now = time.Now()
	}
	expr := Expression{}
	var fuzzy []string

	for _, tok := range tokenize(input) {
		if !strings.Contains(tok, ":") {
			fuzzy = append(fuzzy, tok)
			continue
		}
		key, val, _ := strings.Cut(tok, ":")
		key = strings.ToLower(key)
		val = strings.TrimSpace(val)
		if val == "" {
			continue
		}

		switch key {
		case "state":
			expr.Filter.State = val
		case "assignee":
			expr.Filter.Assignee = resolveUser(val, env)
		case "author":
			expr.Filter.Author = resolveUser(val, env)
		case "label":
			expr.Filter.Labels = append(expr.Filter.Labels, val)
		case "kind":
			switch strings.ToLower(val) {
			case "issue", "issues":
				expr.Kind = "issue"
			case "mr", "mrs", "merge_request":
				expr.Kind = "mr"
			case "both", "any":
				expr.Kind = ""
			}
		case "updated":
			parseUpdated(val, env.Now, &expr)
		default:
			// Unknown key — treat the whole token as fuzzy. Better
			// than dropping it silently when a user typos a key.
			fuzzy = append(fuzzy, tok)
		}
	}

	if len(fuzzy) > 0 {
		expr.Filter.Text = strings.Join(fuzzy, " ")
	}
	return expr
}

// tokenize splits a query string on whitespace, preserving quoted
// strings so `title:"login flow"` works. Quotes are stripped from
// returned tokens.
func tokenize(input string) []string {
	var toks []string
	var cur strings.Builder
	inQuote := false
	flush := func() {
		if cur.Len() > 0 {
			toks = append(toks, cur.String())
			cur.Reset()
		}
	}
	for _, r := range input {
		switch {
		case r == '"':
			inQuote = !inQuote
		case (r == ' ' || r == '\t') && !inQuote:
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return toks
}

// resolveUser expands the `@me` / `@ai` / `@none` / `@any` variables
// and strips `@` from explicit usernames (so users can write
// `assignee:@alice` or `assignee:alice` interchangeably).
func resolveUser(v string, env Env) string {
	if !strings.HasPrefix(v, "@") {
		return v
	}
	switch strings.ToLower(v) {
	case "@me":
		return env.Me
	case "@ai":
		return env.AI
	case "@none":
		return "@none" // sentinel honored by cache.Filter
	case "@any":
		return ""
	default:
		return strings.TrimPrefix(v, "@")
	}
}

// parseUpdated handles values like `>7d`, `<30d`, `=2026-05-01`.
func parseUpdated(v string, now time.Time, out *Expression) {
	if len(v) == 0 {
		return
	}
	op := v[0]
	rest := v
	if op == '>' || op == '<' || op == '=' {
		rest = v[1:]
	} else {
		op = '>'
	}

	d, ok := parseDuration(rest)
	if ok {
		// Relative ago: `>7d` means "updated_at greater than (now-7d)",
		// i.e. updated within the last 7 days.
		anchor := now.Add(-d)
		switch op {
		case '>':
			out.UpdatedAfter = anchor
		case '<':
			out.UpdatedBefore = anchor
		}
		return
	}
	// Try absolute date YYYY-MM-DD.
	if t, err := time.Parse("2006-01-02", rest); err == nil {
		switch op {
		case '>':
			out.UpdatedAfter = t
		case '<':
			out.UpdatedBefore = t
		case '=':
			out.UpdatedAfter = t
			out.UpdatedBefore = t.Add(24 * time.Hour)
		}
	}
}

// parseDuration accepts `7d`, `24h`, `15m`, `30s`. Returns ok=false
// if the format isn't recognized.
func parseDuration(s string) (time.Duration, bool) {
	if len(s) < 2 {
		return 0, false
	}
	unit := s[len(s)-1]
	num := s[:len(s)-1]
	var multiplier time.Duration
	switch unit {
	case 'd':
		multiplier = 24 * time.Hour
	case 'h':
		multiplier = time.Hour
	case 'm':
		multiplier = time.Minute
	case 's':
		multiplier = time.Second
	default:
		return 0, false
	}
	var n int
	for _, r := range num {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
	}
	if n == 0 {
		return 0, false
	}
	return time.Duration(n) * multiplier, true
}
