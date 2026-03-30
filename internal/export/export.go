package export

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/abhishek-rana/lazydk/pkg/messages"
)

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// jsonLine is the structured JSON format for exported log lines.
type jsonLine struct {
	Timestamp string `json:"timestamp"`
	Source    string `json:"source"`
	Container string `json:"container"`
	Level     string `json:"level"`
	Text      string `json:"text"`
	Group     string `json:"group,omitempty"`
}

// LinesToText formats log lines as plain text.
func LinesToText(lines []messages.LogLine) string {
	var b strings.Builder
	for _, line := range lines {
		if !line.Time.IsZero() {
			b.WriteString(line.Time.Format("2006-01-02 15:04:05"))
			b.WriteString(" ")
		}
		if line.SourceID != "" {
			b.WriteString("[")
			b.WriteString(line.SourceID)
			b.WriteString("] ")
		}
		lvl := levelString(line.Level)
		if lvl != "" {
			b.WriteString(lvl)
			b.WriteString(" ")
		}
		b.WriteString(stripANSI(line.Text))
		b.WriteString("\n")
	}
	return b.String()
}

// LinesToJSON formats log lines as newline-delimited JSON.
func LinesToJSON(lines []messages.LogLine) string {
	var b strings.Builder
	for _, line := range lines {
		jl := jsonLine{
			Source:    line.Source,
			Container: line.SourceID,
			Level:     levelString(line.Level),
			Text:      stripANSI(line.Text),
		}
		if !line.Time.IsZero() {
			jl.Timestamp = line.Time.Format(time.RFC3339)
		}
		data, err := json.Marshal(jl)
		if err != nil {
			continue
		}
		b.Write(data)
		b.WriteString("\n")
	}
	return b.String()
}

// ToFile writes content to a file in /tmp and returns the path.
func ToFile(label, content, ext string) (string, error) {
	// Sanitize label for filename.
	safe := strings.NewReplacer("/", "-", " ", "-", ":", "-").Replace(label)
	ts := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("lazydk-%s-%s%s", safe, ts, ext)
	path := filepath.Join(os.TempDir(), filename)

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("writing export file: %w", err)
	}
	return path, nil
}

// ToClipboardOSC52 returns the OSC52 escape sequence to set the terminal clipboard.
func ToClipboardOSC52(content string) string {
	// OSC 52 : set clipboard
	// Format: ESC ] 52 ; c ; <base64-content> ESC \
	encoded := encodeBase64(content)
	return fmt.Sprintf("\033]52;c;%s\033\\", encoded)
}

func encodeBase64(s string) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	data := []byte(s)
	var result strings.Builder

	for i := 0; i < len(data); i += 3 {
		var b0, b1, b2 byte
		b0 = data[i]
		if i+1 < len(data) {
			b1 = data[i+1]
		}
		if i+2 < len(data) {
			b2 = data[i+2]
		}

		result.WriteByte(alphabet[b0>>2])
		result.WriteByte(alphabet[((b0&0x03)<<4)|(b1>>4)])

		if i+1 < len(data) {
			result.WriteByte(alphabet[((b1&0x0F)<<2)|(b2>>6)])
		} else {
			result.WriteByte('=')
		}
		if i+2 < len(data) {
			result.WriteByte(alphabet[b2&0x3F])
		} else {
			result.WriteByte('=')
		}
	}

	return result.String()
}

func levelString(level messages.LogLevel) string {
	switch level {
	case messages.LogLevelDebug:
		return "DEBUG"
	case messages.LogLevelInfo:
		return "INFO"
	case messages.LogLevelWarn:
		return "WARN"
	case messages.LogLevelError:
		return "ERROR"
	case messages.LogLevelFatal:
		return "FATAL"
	default:
		return ""
	}
}

// stripANSI removes ANSI escape codes from a string.
func stripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}
