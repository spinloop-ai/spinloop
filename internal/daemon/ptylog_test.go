package daemon

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"
)

// advance moves the shared test clock by hand.
func (c *fakeClock) advance(d time.Duration) { c.set(c.now().Add(d)) }

// newTestPTYLog builds a normaliser writing to out on the test's clock.
func newTestPTYLog(out *bytes.Buffer, clock *fakeClock) *ptyLog {
	return newPTYLog(out, clock.now)
}

// update is one progress bar redraw, the way the engines draw them: a
// carriage return, the whole line, an erase to the end of it, and the
// carriage return that parks the cursor for the next.
func update(state string) string {
	return "\r" + state + "\033[K\r"
}

// TestPTYLogPlainLines passes a line through as written, CRLF and all: the
// terminal's line ending becomes the log's, and a bare newline commits the
// same way.
func TestPTYLogPlainLines(t *testing.T) {
	out := new(bytes.Buffer)
	clock := &fakeClock{}
	log := newTestPTYLog(out, clock)
	log.Write([]byte("booted\r\nsecond line\n"))
	log.finalize()
	if got := out.String(); got != "booted\nsecond line\n" {
		t.Errorf("plain lines = %q", got)
	}
}

// TestPTYLogBareNewlineIsRecorded keeps the engine's own blank lines: a
// newline is a newline in the record.
func TestPTYLogBareNewlineIsRecorded(t *testing.T) {
	out := new(bytes.Buffer)
	log := newTestPTYLog(out, &fakeClock{})
	log.Write([]byte("one\n\ntwo\n"))
	log.finalize()
	if got := out.String(); got != "one\n\ntwo\n" {
		t.Errorf("a blank line was dropped: %q", got)
	}
}

// TestPTYLogProgressRun is the download: the engine announces itself with a
// newline, then redraws the bar a thousand times. With no time passing at
// all, the log gets the first state and the final one — and nothing between.
func TestPTYLogProgressRun(t *testing.T) {
	out := new(bytes.Buffer)
	log := newTestPTYLog(out, &fakeClock{})
	var b strings.Builder
	b.WriteString("\n") // the bar's own announcement line
	for i := 0; i <= 100; i++ {
		b.WriteString(update(barState(i)))
	}
	log.Write([]byte(b.String()))
	log.finalize()

	want := "\n" + barState(0) + "\n" + barState(100) + "\n"
	if got := out.String(); got != want {
		t.Errorf("the progress run\n got %q\nwant %q", got, want)
	}
	if strings.Contains(out.String(), "\033") {
		t.Errorf("an escape reached the log: %q", out.String())
	}
}

// barState is one state of a test's download bar.
func barState(pct int) string {
	return "Downloading m.gguf " + strings.Repeat("─", pct/2) + " " + fmt.Sprintf("%3d%%", pct)
}

// TestPTYLogProgressThrottled gives the bar a life measured in the interval:
// every state a further one apart lands in the log, so the record reads as
// the progression the download made.
func TestPTYLogProgressThrottled(t *testing.T) {
	out := new(bytes.Buffer)
	clock := &fakeClock{}
	log := newTestPTYLog(out, clock)
	const n = 5
	for i := 0; i < n; i++ {
		if i > 0 {
			clock.advance(ptyRedrawInterval + time.Second)
		}
		log.Write([]byte(update(barState(i * 20))))
	}
	log.finalize()

	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) != n {
		t.Fatalf("the throttled run recorded %d lines, want %d:\n%s", len(lines), n, out.String())
	}
	for i, line := range lines {
		if line != barState(i*20) {
			t.Errorf("line %d = %q, want %q", i, line, barState(i*20))
		}
	}
}

// TestPTYLogIdenticalStatesNeverRepeated: a bar that redraws the same state
// records it once, whatever the interval.
func TestPTYLogIdenticalStatesNeverRepeated(t *testing.T) {
	out := new(bytes.Buffer)
	clock := &fakeClock{}
	log := newTestPTYLog(out, clock)
	log.Write([]byte(update("Downloading m.gguf   4%")))
	clock.advance(ptyRedrawInterval * 10)
	log.Write([]byte(update("Downloading m.gguf   4%")))
	clock.advance(ptyRedrawInterval * 10)
	log.Write([]byte(update("Downloading m.gguf   4%")))
	log.finalize()
	if got := out.String(); got != "Downloading m.gguf   4%\n" {
		t.Errorf("identical states repeated: %q", got)
	}
}

// TestPTYLogEraseShapesTheState: a whole-line erase clears what the state
// holds, the way it clears the terminal's line.
func TestPTYLogEraseShapesTheState(t *testing.T) {
	out := new(bytes.Buffer)
	log := newTestPTYLog(out, &fakeClock{})
	log.Write([]byte(update("abc") + "\033[2K" + update("def") + "\n"))
	log.finalize()
	if got := out.String(); got != "abc\ndef\n" {
		t.Errorf("erase = %q", got)
	}
}

// barUnit is one redraw of a bar drawn the way the engines draw them: the
// bar sits on the line above an anchor the engine keeps returning to, so
// each redraw is a cursor up, the state and its erase, the cursor back
// down, and the carriage return that parks the anchor.
func barUnit(state string) string {
	return "\033[1A" + update(state) + "\033[1B\r"
}

// TestPTYLogRealEngineBarPattern is the single download's whole life: the
// bar announces itself with a newline, then redraws its line a hundred times
// — most of them the same state, the download creeping. The log gets the
// first state and the final one, and the repetition does not repeat.
func TestPTYLogRealEngineBarPattern(t *testing.T) {
	out := new(bytes.Buffer)
	log := newTestPTYLog(out, &fakeClock{})
	var b strings.Builder
	b.WriteString("\n") // the bar's own announcement line
	for i := 0; i < 5; i++ {
		b.WriteString(barUnit(barState(0)))
	}
	b.WriteString(barUnit(barState(100)))
	log.Write([]byte(b.String()))
	log.finalize()

	want := "\n" + barState(0) + "\n" + barState(100) + "\n"
	if got := out.String(); got != want {
		t.Errorf("the progress run\n got %q\nwant %q", got, want)
	}
	if strings.Contains(out.String(), "\033") {
		t.Errorf("an escape reached the log: %q", out.String())
	}
}

// TestPTYLogInterleavedBars: several downloads drawn at once keep their
// states apart — each bar on its own line of the column, each recording its
// own first and final state, the anchor's line among them a blank one.
func TestPTYLogInterleavedBars(t *testing.T) {
	out := new(bytes.Buffer)
	log := newTestPTYLog(out, &fakeClock{})
	stream := "\n" +
		barUnit("bar1  10%") +
		barUnit("bar1  20%") +
		"\n" +
		barUnit("bar2   5%")
	log.Write([]byte(stream))
	log.finalize()
	want := "\nbar1  10%\n\nbar1  20%\nbar2   5%\n"
	if got := out.String(); got != want {
		t.Errorf("interleaved bars = %q, want %q", got, want)
	}
}

// TestPTYLogPendingStateLandsOnTheTick: a state the stream leaves pending —
// the download's last, after which the engine goes quiet on stdout — is not
// left out of the log for the engine's remaining life: the tick records it
// once its time has come, and not before.
func TestPTYLogPendingStateLandsOnTheTick(t *testing.T) {
	out := new(bytes.Buffer)
	clock := &fakeClock{}
	log := newTestPTYLog(out, clock)
	log.Write([]byte(update("Downloading m.gguf   1%")))
	log.tick() // the first state: recorded at once
	if got := out.String(); got != "Downloading m.gguf   1%\n" {
		t.Fatalf("the first state = %q", got)
	}
	clock.advance(ptyRedrawInterval / 2)
	log.Write([]byte(update("Downloading m.gguf 100%")))
	// Under the interval from the last record: the new state waits.
	log.tick()
	if got := out.String(); got != "Downloading m.gguf   1%\n" {
		t.Errorf("the tick recorded before its interval: %q", got)
	}
	clock.advance(ptyRedrawInterval)
	log.tick()
	if got := out.String(); got != "Downloading m.gguf   1%\nDownloading m.gguf 100%\n" {
		t.Errorf("the tick = %q", got)
	}
	// And a tick with nothing pending records nothing again.
	log.tick()
	if got := out.String(); got != "Downloading m.gguf   1%\nDownloading m.gguf 100%\n" {
		t.Errorf("a quiet tick repeated the state: %q", got)
	}
}

// TestPTYLogNoEscapeReachesTheLog: a coloured line loses its colour, keeps
// its words.
func TestPTYLogNoEscapeReachesTheLog(t *testing.T) {
	out := new(bytes.Buffer)
	log := newTestPTYLog(out, &fakeClock{})
	log.Write([]byte("\033[31merror: it failed\033[0m\n"))
	log.finalize()
	if got := out.String(); got != "error: it failed\n" {
		t.Errorf("colour = %q", got)
	}
	if strings.Contains(out.String(), "\033") {
		t.Errorf("an escape reached the log: %q", out.String())
	}
}

// TestPTYLogTrailingCarriageReturnKeepsTheLine: a carriage return at the end
// of a plain line moves the cursor, it does not erase the line.
func TestPTYLogTrailingCarriageReturnKeepsTheLine(t *testing.T) {
	out := new(bytes.Buffer)
	log := newTestPTYLog(out, &fakeClock{})
	log.Write([]byte("hello\r"))
	log.finalize()
	if got := out.String(); got != "hello\n" {
		t.Errorf("trailing CR = %q", got)
	}
}

// TestPTYLogTrailingCarriageReturnAfterTheFinalState: the bar's own trailing
// carriage return does not withhold the state it follows.
func TestPTYLogTrailingCarriageReturnAfterTheFinalState(t *testing.T) {
	out := new(bytes.Buffer)
	log := newTestPTYLog(out, &fakeClock{})
	log.Write([]byte(update("Downloading m.gguf 100%")))
	log.finalize()
	if got := out.String(); got != "Downloading m.gguf 100%\n" {
		t.Errorf("trailing CR after the final state = %q", got)
	}
}

// TestPTYLogUnnewlineledLineAtEndOfStream: output the engine never finished
// with a newline is the record's to keep.
func TestPTYLogUnnewlineledLineAtEndOfStream(t *testing.T) {
	out := new(bytes.Buffer)
	log := newTestPTYLog(out, &fakeClock{})
	log.Write([]byte("loading model"))
	log.finalize()
	if got := out.String(); got != "loading model\n" {
		t.Errorf("unnewlineled = %q", got)
	}
}

// TestPTYLogSplitStateRecordsWhole: a state that arrives over several reads
// is recorded once, at the end, never half-drawn in the middle.
func TestPTYLogSplitStateRecordsWhole(t *testing.T) {
	out := new(bytes.Buffer)
	log := newTestPTYLog(out, &fakeClock{})
	log.Write([]byte("\rHalf"))
	log.Write([]byte("Done"))
	log.Write([]byte("\n"))
	log.finalize()
	if got := out.String(); got != "HalfDone\n" {
		t.Errorf("split state = %q", got)
	}
}

// TestPTYLogPumpDrainsAndFinalises runs the pump over a finished stream: the
// log gets the plain line and the bar's states, the final one among them, and
// done closes.
func TestPTYLogPumpDrainsAndFinalises(t *testing.T) {
	stream := "booted\r\n" + update("bar  1%") + update("bar  2%")
	out := new(bytes.Buffer)
	done := make(chan struct{})
	pumpPTYLog(strings.NewReader(stream), out, done)
	<-done
	want := "booted\nbar  1%\nbar  2%\n"
	if got := out.String(); got != want {
		t.Errorf("the pump = %q, want %q", got, want)
	}
}
