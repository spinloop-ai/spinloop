package daemon

import (
	"io"
	"sync"
	"time"
)

// ptyRedrawInterval is the frequency a redrawing line's states are recorded
// at while the redraw goes on. It sits under the serve view's own poll
// cadence, so the pane sees fresh states as they land, and bounds a chatty
// bar — a download makes a thousand updates — to a legible run of lines.
const ptyRedrawInterval = 2 * time.Second

// ptyLog turns a captured engine's terminal output — the bytes the
// pseudo-terminal's master hands over — into clean lines for the engine log.
//
// It keeps the terminal's lines as a column, the way the screen holds them,
// and a line the engine redraws in place — a download progress bar among
// them — is recorded as its state rather than as a run of raw overwrites.
// The rules:
//
//   - The terminal's CRLF line ending becomes the log's LF, and a bare LF
//     commits the line the same way. Committing a line records it — a plain
//     line as written, a redrawing line as its final state — and moves the
//     drawing to the line below; the committed line stays where it is, and
//     a cursor move may come back to it.
//   - A carriage return that is not a line ending starts a new state of the
//     line the engine is drawing: what follows replaces the state, it does
//     not append to it. A state is complete when the engine moves on — a
//     new state, a newline, an erase, or the end of the stream — never in
//     the middle of its bytes, so a state is never recorded half-drawn.
//   - A redrawing line is recorded as its state: the first state always, the
//     final state always, and a further distinct state at most once per
//     ptyRedrawInterval. Identical states are never repeated, and a state
//     that is empty is never recorded — a blank line is a drawing, not
//     output.
//   - Cursor moves do not end a line: up and down move the drawing between
//     the column's lines, which is how the engines redraw a bar in place —
//     up to its line, the state, the cursor back down to the anchor. A home
//     returns the drawing to the top of the column.
//   - Erase sequences shape the state the way they shape the terminal's
//     line: a whole-line erase ends the state drawn on it and clears what
//     it holds; an erase to the end of the line erases from the cursor,
//     which sits at the end of the state, and so changes nothing.
//   - A plain line — one the engine never redrew — is recorded as written,
//     when its newline or the end of the stream commits it.
//   - Every other escape is dropped, and one that is not part of the
//     redraw's own unit ends the state drawn before it. No escape reaches
//     the log.
type ptyLog struct {
	// mu guards the state below: the pump feeds it from its own goroutine
	// while the tick's runs its own, and a state the tick records and a
	// line the pump commits are the same fields.
	mu       sync.Mutex
	w        io.Writer
	now      func() time.Time
	interval time.Duration

	lines []*ptyLine // the screen's lines, top down; the drawing is on lines[row]
	row   int

	esc      int  // the escape sequence's state; escNone among plain bytes
	csiParam byte // the last parameter of the CSI sequence being read
}

// ptyLine is one line of the screen the normaliser keeps.
type ptyLine struct {
	content   []byte // the content of the line as drawn
	drawing   bool   // the line is one the engine is redrawing
	pendingCR bool   // a carriage return is held, to tell CRLF apart from a redraw
	replacing bool   // a held carriage return settled as a redraw: the next
	// text byte starts a new state, replacing the content the line held
	committed bool // the line is recorded and untouched since

	recorded bool   // a state of the redraw has been recorded
	last     []byte // the last recorded state
	lastAt   time.Time
}

// The escape sequence's states.
const (
	escNone  = iota
	escStart // after the escape itself
	escCSI   // after the intro: the parameter bytes, then a final
	escOSC   // after the OSC intro: consumed to the BEL or the ST
	escOSCST // the ST's escape, waiting on its backslash
)

// newPTYLog builds the normaliser that writes its lines to w. now is
// injected so the frequency rule's boundary is testable without a clock.
func newPTYLog(w io.Writer, now func() time.Time) *ptyLog {
	return &ptyLog{w: w, now: now, interval: ptyRedrawInterval, lines: []*ptyLine{{}}}
}

// line is the line the drawing is on.
func (p *ptyLog) line() *ptyLine {
	return p.lines[p.row]
}

// extend makes the column hold at least up to the row: the lines between
// are empty, the way the screen's are.
func (p *ptyLog) extend(row int) {
	for len(p.lines) <= row {
		p.lines = append(p.lines, &ptyLine{})
	}
}

// Write feeds one read of the pseudo-terminal to the normaliser. It always
// accepts the bytes — the pump must keep draining the terminal whatever the
// log's fate — and reports them all as written.
func (p *ptyLog) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, c := range b {
		p.byte(c)
	}
	return len(b), nil
}

// byte is the normaliser's one step: the escape sequence's states first,
// then the line's.
func (p *ptyLog) byte(c byte) {
	switch p.esc {
	case escStart:
		switch c {
		case '[':
			p.esc, p.csiParam = escCSI, 0
		case ']':
			p.esc = escOSC
		default:
			// A two-byte sequence the log has no business in: the
			// state drawn before it is done, and both bytes are
			// dropped.
			p.settleOnEscape()
			p.esc = escNone
		}
		return
	case escCSI:
		switch {
		case c >= 0x30 && c <= 0x3f: // parameter bytes
			if c >= 0x30 && c <= 0x39 {
				p.csiParam = c
			}
		case c >= 0x20 && c <= 0x2f: // intermediate bytes
		case c >= 0x40 && c <= 0x7e: // the final byte decides what happens
			p.csiFinal(c)
			p.esc = escNone
		default:
			p.settleOnEscape() // malformed: the state is done, the bytes dropped
			p.esc = escNone
		}
		return
	case escOSC:
		switch c {
		case 0x07: // the BEL ends the sequence
			p.settleOnEscape()
			p.esc = escNone
		case 0x1b: // the ST's escape
			p.esc = escOSCST
		}
		return
	case escOSCST:
		// Whether the backslash arrives or not, the sequence is over.
		p.settleOnEscape()
		p.esc = escNone
		return
	}

	l := p.line()
	switch c {
	case 0x1b:
		// What the escape does to the line's state is decided when the
		// sequence ends.
		p.esc = escStart
	case '\r':
		if l.pendingCR {
			// A second carriage return: the first settled as a redraw
			// start, and this one holds in its turn.
			p.settle(l)
			l.pendingCR = false
			l.replacing = true
		}
		l.pendingCR = true
	case '\n':
		// Whether or not a carriage return held, a newline is a line
		// ending: whatever state the line holds stands, and is
		// committed with it.
		l.pendingCR = false
		l.replacing = false
		p.commit()
	default:
		if l.pendingCR {
			// A held carriage return that is not a line ending: the
			// state drawn before it, if any, is complete, and a new
			// state of the line begins.
			p.settle(l)
			l.pendingCR = false
			l.replacing = true
			l.drawing = true
		}
		if l.replacing {
			l.content = l.content[:0]
			l.replacing = false
		}
		l.content = append(l.content, c)
		l.committed = false
	}
}

// settle is a state boundary on the line: the state drawn before it, if
// any, is complete. It does not touch the content: a redraw's replacement
// is deferred to the text byte that starts the new state — or the erase
// that takes the state away — so a state the engine drew stands when a
// newline ends the line, the way it stands on the screen.
func (p *ptyLog) settle(l *ptyLine) {
	if l.drawing {
		p.considerState(l)
	}
}

// settleOnEscape settles the line's held carriage return when the escape
// that just finished is not part of the redraw's own unit — the erase and
// the cursor moves are: they end or continue the drawing, and the state
// stands until one of them says otherwise.
func (p *ptyLog) settleOnEscape() {
	if l := p.line(); l.pendingCR {
		p.settle(l)
		l.pendingCR = false
		l.replacing = true
	}
}

// csiCount is the parameter of a CSI sequence's count, the way the
// terminal reads it: absent or zero means one.
func (p *ptyLog) csiCount() int {
	if p.csiParam >= '1' && p.csiParam <= '9' {
		return int(p.csiParam - '0')
	}
	return 1
}

// csiFinal is what a CSI sequence's final byte does to the drawing.
func (p *ptyLog) csiFinal(c byte) {
	switch c {
	case 'A': // the drawing moves up the column; the line it leaves stands
		if n := p.csiCount(); p.row >= n {
			p.row -= n
		} else {
			p.row = 0
		}
	case 'B': // the drawing moves down
		p.row += p.csiCount()
		p.extend(p.row)
	case 'H': // home: the top of the column
		p.row = 0
	case 'K':
		if p.csiParam == '2' { // the whole line is erased: the state drawn is done
			l := p.line()
			p.settle(l)
			l.pendingCR = false
			l.replacing = false
			l.content = l.content[:0]
		}
		// An erase to the end erases from the cursor, which sits at the
		// end of the state: nothing of the log's to clear, and the state
		// is not done for it.
	case 'J': // an erase past the line's end clears what it holds
		l := p.line()
		p.settle(l)
		l.pendingCR = false
		l.replacing = false
		l.content = l.content[:0]
	default:
		// A colour, a position the log does not model, a mode: the
		// state drawn before the escape is done.
		p.settleOnEscape()
	}
}

// commit ends the current line, as a newline does: a plain line is recorded
// as written — an empty one as a blank line, a bare newline's record — and
// a redrawing line records its final state. The line then stays in place;
// the drawing moves to the line below it.
func (p *ptyLog) commit() {
	p.commitLine(p.line(), false)
	p.row++
	p.extend(p.row)
}

// commitLine records one line's content, the way a commit does, and marks
// it so the end of the stream does not record it a second time. A line
// touched again after its commit is recorded again, as the terminal shows
// it: corrected, not duplicated.
func (p *ptyLog) commitLine(l *ptyLine, atEOF bool) {
	if l.committed {
		return
	}
	switch {
	case l.drawing:
		p.recordFinal(l, l.content)
	case len(l.content) > 0:
		p.writeBytes(l.content)
	case !atEOF:
		p.writeBytes(l.content) // the blank line
	}
	l.committed = true
}

// considerState records the line's current state when the frequency rule
// allows: the first state always, a further distinct state at most once per
// the interval.
func (p *ptyLog) considerState(l *ptyLine) {
	if len(l.content) == 0 {
		return
	}
	if l.recorded && string(l.content) == string(l.last) {
		return
	}
	if !l.recorded || p.now().Sub(l.lastAt) >= p.interval {
		p.writeBytes(l.content)
		l.last = append([]byte(nil), l.content...)
		l.lastAt = p.now()
		l.recorded = true
	}
}

// tick records a redrawing line's pending state whose time has come, on
// every line of the column: a bar's last state outlives the engine's move
// off its line, and the log owes it the state anyway. The pump runs it at
// the interval's half-rate for the engine's life; the dedup and the
// interval make it a no-op on a line that has nothing pending, which is the
// normal state.
func (p *ptyLog) tick() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, l := range p.lines {
		if l.drawing {
			p.considerState(l)
		}
	}
}

// finalize records whatever the stream leaves unfinished: each line of the
// column, top down, a redrawing line its final state — unthrottled, since
// nothing follows it — and a plain line its content. A held carriage return
// moves the cursor, it does not erase, so whatever state is drawn stands.
func (p *ptyLog) finalize() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, l := range p.lines {
		l.pendingCR = false
		p.commitLine(l, true)
	}
}

// recordFinal records the redraw's final state, unthrottled, unless it is
// the state the log already carries for the line.
func (p *ptyLog) recordFinal(l *ptyLine, b []byte) {
	if len(b) == 0 {
		return
	}
	if l.recorded && string(b) == string(l.last) {
		return
	}
	p.writeBytes(b)
	l.last = append([]byte(nil), b...)
	l.lastAt = p.now()
	l.recorded = true
}

// writeBytes appends one line to the log, newline and all. A write that
// fails is swallowed: the log's fate is not the engine's, and the pump must
// keep draining the terminal.
func (p *ptyLog) writeBytes(line []byte) {
	buf := append(append([]byte(nil), line...), '\n')
	p.w.Write(buf)
}

// pumpPTYLog reads the pseudo-terminal's master for the engine's life,
// normalising its stream into the log's lines on out. It always drains —
// the frequency rule delays the log's writes, never the read, so the engine
// can never wedge on a full terminal — and it records the final state before
// it closes done.
func pumpPTYLog(master io.Reader, out io.Writer, done chan<- struct{}) {
	log := newPTYLog(out, time.Now)
	stopping := make(chan struct{})
	go func() {
		ticker := time.NewTicker(ptyRedrawInterval / 2)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				log.tick()
			case <-stopping:
				return
			}
		}
	}()
	buf := make([]byte, 32<<10)
	for {
		n, err := master.Read(buf)
		if n > 0 {
			log.Write(buf[:n])
		}
		if err == nil {
			continue
		}
		// The end of the stream: on some platforms the error arrives
		// before the last bytes do, so read until it stops coming.
		for {
			n, err := master.Read(buf)
			if n > 0 {
				log.Write(buf[:n])
			}
			if err != nil {
				break
			}
		}
		break
	}
	log.finalize()
	close(stopping)
	close(done)
}
