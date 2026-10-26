//go:build !windows

package daemon

import (
	"bytes"
	"strings"
	"testing"
)

func TestLineTeeHandsOverCompleteLines(t *testing.T) {
	var file bytes.Buffer
	var lines []string
	tee := &lineTee{w: &file, onLine: func(l string) { lines = append(lines, l) }}

	// One write, three lines and a partial remainder: the lines land, the
	// remainder waits for its newline.
	n, err := tee.Write([]byte("booting\nready\n"))
	if err != nil || n != 14 {
		t.Fatalf("Write = %d, %v; want 14 bytes, nil", n, err)
	}
	if _, err := tee.Write([]byte("tg =   3.87 t/s, tg_3s =   5.29 t/s\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := tee.Write([]byte("half a li")); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(lines, "|"); got != "booting|ready|tg =   3.87 t/s, tg_3s =   5.29 t/s" {
		t.Errorf("lines = %q, want the three complete ones", got)
	}

	// The remainder joins its own continuation and lands on its newline.
	if _, err := tee.Write([]byte("ne\n")); err != nil {
		t.Fatal(err)
	}
	if lines[len(lines)-1] != "half a line" {
		t.Errorf("last line = %q, want the joined one", lines[len(lines)-1])
	}

	// The file carries every byte exactly — newlines included.
	if file.String() != "booting\nready\ntg =   3.87 t/s, tg_3s =   5.29 t/s\nhalf a line\n" {
		t.Errorf("file = %q, want every byte", file.String())
	}
}

func TestLineTeeDropsAnOverlongLine(t *testing.T) {
	var file bytes.Buffer
	var lines []string
	tee := &lineTee{w: &file, onLine: func(l string) { lines = append(lines, l) }}

	// A line past the cap is dropped whole, from here to its newline; the
	// next line is read clean.
	filler := strings.Repeat("x", teeLineCap+1)
	if _, err := tee.Write([]byte("runaway " + filler + "\ngood\n")); err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 || lines[0] != "good" {
		t.Errorf("lines = %d %q, want only the line after the runaway", len(lines), lines)
	}

	// And a cap bust that spans writes still drops to its newline, without
	// handing over a fabricated half-line at the break.
	if _, err := tee.Write([]byte(strings.Repeat("y", teeLineCap-2))); err != nil {
		t.Fatal(err)
	}
	if _, err := tee.Write([]byte("tail\nsecond\n")); err != nil {
		t.Fatal(err)
	}
	if len(lines) != 2 || lines[1] != "second" {
		t.Errorf("lines = %q, want the runaway dropped and 'second' read", lines)
	}
}

func TestLineTeeHandsNothingToANewlinelessEnd(t *testing.T) {
	var file bytes.Buffer
	fed := false
	tee := &lineTee{w: &file, onLine: func(string) { fed = true }}
	if _, err := tee.Write([]byte("no newline at all")); err != nil {
		t.Fatal(err)
	}
	if fed {
		t.Error("an unterminated line was handed over")
	}
}
