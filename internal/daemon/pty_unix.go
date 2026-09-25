//go:build !windows

package daemon

import (
	"os"

	"github.com/creack/pty"
)

// ptyWindow is the window a captured engine's pseudo-terminal reports. Wide
// enough that an engine sizes its terminal output for an ordinary screen — a
// download bar among them — and narrow enough that the recorded lines stay
// compact in the view's pane and for the log's other readers.
var ptyWindow = &pty.Winsize{Rows: 50, Cols: 80}

// attachPTY opens a pseudo-terminal for a captured engine's stdout: the
// master is what the supervisor reads, the slave is what the engine's stdout
// holds. An error means no pseudo-terminal could be opened — the unsupported
// platform among them — and the capture falls back to the log file.
func attachPTY() (master, slave *os.File, err error) {
	master, slave, err = pty.Open()
	if err != nil {
		return nil, nil, err
	}
	if err := pty.Setsize(master, ptyWindow); err != nil {
		master.Close()
		slave.Close()
		return nil, nil, err
	}
	return master, slave, nil
}
