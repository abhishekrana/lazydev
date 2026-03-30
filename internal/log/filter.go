package log

import (
	"regexp"
	"strings"

	"github.com/abhishek-rana/lazydk/pkg/messages"
)

// Filter specifies criteria for matching log lines. An empty/zero-value
// Filter matches everything.
type Filter struct {
	Query string
	Level messages.LogLevel
	Regex *regexp.Regexp
}

// NewFilter creates a Filter. If query contains regex metacharacters
// and can be compiled, it is stored as a compiled Regex. Otherwise
// plain substring matching is used via Query.
func NewFilter(query string, level messages.LogLevel) Filter {
	f := Filter{
		Query: query,
		Level: level,
	}

	if query != "" && containsRegexMeta(query) {
		if re, err := regexp.Compile(query); err == nil {
			f.Regex = re
		}
	}

	return f
}

// Match returns true if the given log line satisfies the filter.
// An empty filter matches everything.
func (f Filter) Match(line messages.LogLine) bool {
	// Level filter: if set, the line must be at or above the requested level.
	if f.Level != messages.LogLevelUnknown && line.Level < f.Level {
		return false
	}

	// Text filter.
	if f.Query != "" {
		if f.Regex != nil {
			if !f.Regex.MatchString(line.Text) {
				return false
			}
		} else {
			if !strings.Contains(strings.ToLower(line.Text), strings.ToLower(f.Query)) {
				return false
			}
		}
	}

	return true
}

// containsRegexMeta returns true if s contains characters that suggest
// it is intended as a regular expression rather than a plain string.
func containsRegexMeta(s string) bool {
	for _, c := range s {
		switch c {
		case '\\', '^', '$', '.', '|', '?', '*', '+', '(', ')', '[', '{':
			return true
		}
	}
	return false
}
