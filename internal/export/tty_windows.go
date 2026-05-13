//go:build windows

package export

import (
	"fmt"
	"os"
)

func openTTY() (*os.File, error) {
	return nil, fmt.Errorf("/dev/tty not available on Windows; clipboard via OSC52 unsupported")
}
