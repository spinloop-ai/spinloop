//go:build windows

package daemon

import (
	"errors"
	"os"
)

// attachPTY has no pseudo-terminal on Windows: the capture falls back to the
// log file, no worse than it did before the pseudo-terminal existed.
func attachPTY() (*os.File, *os.File, error) {
	return nil, nil, errors.New("no pseudo-terminal on this platform")
}
