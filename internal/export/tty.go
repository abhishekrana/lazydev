//go:build !windows

package export

import "os"

// openTTY returns a writable handle to the controlling terminal so we
// can emit OSC52 sequences without interfering with Bubble Tea's
// stdout buffering.
func openTTY() (*os.File, error) {
	return os.OpenFile("/dev/tty", os.O_WRONLY, 0)
}
