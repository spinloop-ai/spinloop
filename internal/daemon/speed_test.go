//go:build !windows

package daemon

import (
	"context"
	"testing"
	"time"

	"github.com/spinloop-ai/spinloop/internal/inference"
)

func TestEngineSpeedFoldAndLapse(t *testing.T) {
	var s engineSpeed

	// Nothing observed: neither a point nor a figure.
	if p := s.sampleTick(baseTime); p != nil {
		t.Errorf("a tick with no figure returned %v, want none", p)
	}
	if c := s.current(baseTime, time.Second); c != nil {
		t.Errorf("a read with no figure returned %v, want none", c)
	}

	// A figure observed: the next tick folds it once, and the one after
	// that saw nothing new, so it folds nothing.
	s.observe(12.5, baseTime.Add(time.Second))
	if p := s.sampleTick(baseTime.Add(15 * time.Second)); p == nil || *p != 12.5 {
		t.Errorf("folded point = %v, want 12.5", p)
	}
	if p := s.sampleTick(baseTime.Add(30 * time.Second)); p != nil {
		t.Errorf("a quiet interval folded %v, want none", p)
	}
	// A newer figure replaces the older on the next fold.
	s.observe(9.0, baseTime.Add(31*time.Second))
	if p := s.sampleTick(baseTime.Add(45 * time.Second)); p == nil || *p != 9.0 {
		t.Errorf("folded point = %v, want the newest figure", p)
	}

	// The current figure stands for an interval's worth of quiet and lapses
	// past it — the same clock the fold runs on.
	s.observe(7.25, baseTime.Add(50*time.Second))
	if c := s.current(baseTime.Add(55*time.Second), 15*time.Second); c == nil || *c != 7.25 {
		t.Errorf("current = %v, want 7.25", c)
	}
	if c := s.current(baseTime.Add(70*time.Second), 15*time.Second); c != nil {
		t.Errorf("an aged figure still reads %v, want none", c)
	}

	// A forgotten record answers nothing: one engine's rate is not the next
	// engine's.
	s.observe(3.0, baseTime.Add(80*time.Second))
	s.forget()
	if p := s.sampleTick(baseTime.Add(95 * time.Second)); p != nil {
		t.Errorf("after forget a tick folded %v, want none", p)
	}
	if c := s.current(baseTime.Add(95*time.Second), time.Hour); c != nil {
		t.Errorf("after forget the current figure reads %v, want none", c)
	}
}

// TestSpeedEndToEnd runs a fake engine that prints llama.cpp's timing line,
// and reads the figure back through the capture hook, the sampler fold and
// the metrics reply — every seam in one wire.
func TestSpeedEndToEnd(t *testing.T) {
	d := testDaemon(t, `trap 'exit 0' TERM
echo "86.15.860.290 I slot print_timing: id  0 | task 326 | n_gen =   9438, tg =   3.87 t/s, tg_3s =   5.29 t/s" >&2
while true; do sleep 0.05; done`)
	d.Now = func() time.Time { return baseTime }
	d.Collector = linuxCollector()

	if err := d.Push(inference.DeployConfig{Runner: "llamacpp", ModelID: "m"}); err != nil {
		t.Fatal(err)
	}
	if err := d.StartEngine(); err != nil {
		t.Fatal(err)
	}
	defer d.Sup.Stop()
	waitForState(t, d.Sup, StateRunning)

	// The hook fires when the line lands, not when the test asks; wait for
	// the figure the way a real sampler waits for a real engine.
	waitForSpeed(t, d)

	// The tick folds the figure into the history.
	d.systemSampleOnce(context.Background())
	hist := d.hist.snapshot()
	if len(hist) != 1 {
		t.Fatalf("history = %d samples, want 1", len(hist))
	}
	if hist[0].Speed == nil || *hist[0].Speed != 3.87 {
		t.Errorf("sample speed = %v, want 3.87", hist[0].Speed)
	}

	// The running metrics reply carries the current figure while it is fresh
	// (the clock is pinned at baseTime, and the figure is observed "now").
	stats := d.Metrics(context.Background())
	if stats.Speed == nil || *stats.Speed != 3.87 {
		t.Fatalf("metrics speed = %v, want 3.87", stats.Speed)
	}

	// A tick's worth of quiet later — same figure in hand, no new one — the
	// current reading has lapsed and the next tick folds nothing, while the
	// point already taken stays in the window.
	d.Now = func() time.Time { return baseTime.Add(time.Minute) }
	if got := d.Metrics(context.Background()).Speed; got != nil {
		t.Errorf("a quiet minute still reads %v, want the figure lapsed", got)
	}
	d.systemSampleOnce(context.Background())
	hist = d.hist.snapshot()
	if len(hist) != 2 || hist[1].Speed != nil {
		t.Errorf("a quiet tick folded %+v, want a second sample with no speed point", hist[1:])
	}

	// The stop keeps the history's point and drops the current figure.
	d.Sup.Stop()
	waitForState(t, d.Sup, StateStopped)
	stats = d.Metrics(context.Background())
	if stats.Speed != nil {
		t.Errorf("a stopped engine still reports %v, want no current figure", stats.Speed)
	}
	hist = d.hist.snapshot()
	if len(hist) != 2 || hist[0].Speed == nil || *hist[0].Speed != 3.87 {
		t.Errorf("history after a stop = %+v, want the taken point retained", hist)
	}
}

// TestSpeedSampleExistsOnSpeedAlone pins the fold's presence rule: a tick
// with a figure and no host figures at all is still a sample — the rate is
// an observation in its own right.
func TestSpeedSampleExistsOnSpeedAlone(t *testing.T) {
	d := testDaemon(t, `trap 'exit 0' TERM
while true; do sleep 0.05; done`)
	d.Now = func() time.Time { return baseTime }
	// No collector at all: the host yields nothing, ever.

	if err := d.Push(inference.DeployConfig{Runner: "llamacpp", ModelID: "m"}); err != nil {
		t.Fatal(err)
	}
	if err := d.StartEngine(); err != nil {
		t.Fatal(err)
	}
	defer d.Sup.Stop()
	waitForState(t, d.Sup, StateRunning)

	// A tick with nothing seen adds nothing.
	d.systemSampleOnce(context.Background())
	if got := d.hist.snapshot(); len(got) != 0 {
		t.Fatalf("an empty tick retained %+v, want nothing", got)
	}

	// The same tick with a figure retained a sample — the speed point alone.
	// The clock advances a moment first: with it pinned, the figure would
	// land on the empty tick's own instant, and "since the last tick" is a
	// strict boundary.
	d.Now = func() time.Time { return baseTime.Add(time.Second) }
	d.ObserveEngineLine("slot print_timing: id  0 | task 3 | tg = 6.10 t/s")
	d.systemSampleOnce(context.Background())
	hist := d.hist.snapshot()
	if len(hist) != 1 || hist[0].Speed == nil || *hist[0].Speed != 6.10 {
		t.Fatalf("history = %+v, want one speed-only sample", hist)
	}
	if hist[0].CPU != nil || hist[0].Mem != nil || len(hist[0].GPUs) != 0 {
		t.Errorf("the speed-only sample carried host figures: %+v", hist[0])
	}
}

// TestSpeedForgetsOnStart pins the engine boundary: the second engine's
// report answers with its own rates or none, never the first engine's.
func TestSpeedForgetsOnStart(t *testing.T) {
	d := testDaemon(t, `trap 'exit 0' TERM
while true; do sleep 0.05; done`)
	d.Now = func() time.Time { return baseTime }

	if err := d.Push(inference.DeployConfig{Runner: "llamacpp", ModelID: "m"}); err != nil {
		t.Fatal(err)
	}
	d.ObserveEngineLine("slot print_timing: id  0 | task 1 | tg = 4.00 t/s")

	if err := d.StartEngine(); err != nil {
		t.Fatal(err)
	}
	waitForState(t, d.Sup, StateRunning)
	if got := d.Metrics(context.Background()).Speed; got != nil {
		d.Sup.Stop()
		t.Fatalf("the new engine reports %v, want the previous figure forgotten", got)
	}
	d.systemSampleOnce(context.Background())
	if got := d.hist.snapshot(); len(got) != 0 {
		d.Sup.Stop()
		t.Fatalf("the new engine's first tick folded %+v, want nothing", got)
	}
	d.Sup.Stop()
	waitForState(t, d.Sup, StateStopped)
}

func waitForSpeed(t *testing.T, d *Daemon) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		// A generous window: this is waiting for the line to arrive, not
		// testing the lapse — the pinned clock keeps the figure observed at
		// baseTime, and real seconds have passed since the wall-clock start.
		if got := d.speed.current(baseTime, 24*time.Hour); got != nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the engine's timing line never reached the tracker")
}
