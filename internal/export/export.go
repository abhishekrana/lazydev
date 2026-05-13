// Package export provides clipboard (OSC52) and tempfile helpers
// used by the Claude context-export feature.
package export

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ToFile writes content to a file in /tmp and returns the path.
func ToFile(label, content, ext string) (string, error) {
	safe := strings.NewReplacer("/", "-", " ", "-", ":", "-").Replace(label)
	ts := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("lazydev-%s-%s%s", safe, ts, ext)
	path := filepath.Join(os.TempDir(), filename)

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("writing export file: %w", err)
	}
	return path, nil
}

// ToClipboardOSC52 returns the OSC52 escape sequence to set the terminal
// clipboard. Print the returned string to the controlling tty; works over
// SSH and tmux when the host terminal supports OSC52.
func ToClipboardOSC52(content string) string {
	return fmt.Sprintf("\033]52;c;%s\033\\", encodeBase64(content))
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
