package daemon

import (
	"bytes"
	"io"
	"sync"
)

// teeLineCap bounds the line tee's partial-line buffer. The lines worth
// reading are log lines, and a generator that prints a megabyte without a
// newline is not producing any; past the cap the rest of that line is
// dropped rather than buffered, so a runaway stream costs a fixed few
// kilobytes and never grows with the stream.
const teeLineCap = 64 << 10

// lineTee is the supervisor's capture sink when an OnOutput hook is wired:
// every byte flows to the log file untouched, and every complete line —
// bytes up to a newline, the newline not included — is additionally handed
// to the hook. The file still receives the exact bytes; the hook only ever
// sees lines whole, so the parser is never asked to read half a figure.
//
// Writes arrive from the pty pump's goroutine and the stderr copier's at
// once, so the buffer sits under a mutex. A line longer than the cap is
// dropped from that newline to the next: a partial line the tee hands over
// would be a fabricated one.
type lineTee struct {
	w      io.Writer
	onLine func(string)

	mu   sync.Mutex
	buf  []byte
	skip bool // the current line busted the cap; drop it to its newline
}

func (t *lineTee) Write(p []byte) (int, error) {
	n, err := t.w.Write(p)
	t.mu.Lock()
	defer t.mu.Unlock()
	for len(p) > 0 {
		i := bytes.IndexByte(p, '\n')
		if i < 0 {
			if t.skip {
				break
			}
			if len(t.buf)+len(p) > teeLineCap {
				t.buf, t.skip = t.buf[:0], true
				break
			}
			t.buf = append(t.buf, p...)
			break
		}
		if !t.skip {
			if len(t.buf)+len(p[:i]) > teeLineCap {
				// The line busts the cap with its own remainder: drop it
				// whole rather than hand over a truncated half-line.
				t.skip = true
			} else {
				t.buf = append(t.buf, p[:i]...)
				if len(t.buf) > 0 {
					t.onLine(string(t.buf))
				}
			}
		}
		t.buf, t.skip = t.buf[:0], false
		p = p[i+1:]
	}
	return n, err
}
