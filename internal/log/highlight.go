package log

import (
	"strings"
	"time"

	"github.com/abhishek-rana/lazydk/pkg/messages"
)

// ParseLogLevel scans text for common log-level keywords and returns
// the corresponding LogLevel. Matching is case-insensitive.
// If no known level keyword is found, LogLevelUnknown is returned.
func ParseLogLevel(text string) messages.LogLevel {
	upper := strings.ToUpper(text)

	// Order matters: check more specific / severe levels first so that
	// a line containing both "INFO" and "ERROR" is classified as ERROR.
	switch {
	case strings.Contains(upper, "PANIC"):
		return messages.LogLevelFatal
	case strings.Contains(upper, "FATAL"):
		return messages.LogLevelFatal
	case strings.Contains(upper, "ERROR"):
		return messages.LogLevelError
	case strings.Contains(upper, "WARNING"):
		return messages.LogLevelWarn
	case strings.Contains(upper, "WARN"):
		return messages.LogLevelWarn
	case strings.Contains(upper, "INFO"):
		return messages.LogLevelInfo
	case strings.Contains(upper, "DEBUG"):
		return messages.LogLevelDebug
	default:
		return messages.LogLevelUnknown
	}
}

// timestampFormats lists common timestamp layouts to try, ordered from
// most specific to least specific.
var timestampFormats = []string{
	time.RFC3339Nano,                // 2006-01-02T15:04:05.999999999Z07:00
	time.RFC3339,                    // 2006-01-02T15:04:05Z07:00
	"2006-01-02T15:04:05.000Z0700", // ISO 8601 variant
	"2006-01-02T15:04:05",          // ISO 8601 without timezone
	"2006-01-02 15:04:05.000",      // Common log format with millis
	"2006-01-02 15:04:05",          // Common log format
	time.Stamp,                     // Jan _2 15:04:05  (syslog)
	time.StampMilli,                // Jan _2 15:04:05.000
	time.StampMicro,                // Jan _2 15:04:05.000000
	time.StampNano,                 // Jan _2 15:04:05.000000000
	time.DateTime,                  // 2006-01-02 15:04:05
	time.ANSIC,                     // Mon Jan _2 15:04:05 2006
	time.UnixDate,                  // Mon Jan _2 15:04:05 MST 2006
	time.RubyDate,                  // Mon Jan 02 15:04:05 -0700 2006
	"02/Jan/2006:15:04:05 -0700",   // Apache/nginx combined log
	"Jan 02 15:04:05",              // syslog without year
}

// ParseTimestamp attempts to extract a timestamp from the beginning of
// text using common formats. It returns the zero time if no format matches.
func ParseTimestamp(text string) time.Time {
	trimmed := strings.TrimSpace(text)

	for _, layout := range timestampFormats {
		// Try parsing from the start of the string. We only need the
		// prefix to match, so we truncate the input to the layout length
		// plus some slack for timezone info.
		end := len(layout) + 10
		if end > len(trimmed) {
			end = len(trimmed)
		}
		candidate := trimmed[:end]

		if t, err := time.Parse(layout, candidate); err == nil {
			return t
		}
	}

	return time.Time{}
}
