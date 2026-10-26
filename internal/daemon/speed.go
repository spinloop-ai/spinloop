package daemon

import (
	"sync"
	"time"

	"github.com/spinloop-ai/spinloop/internal/metrics"
)

// engineSpeed is the daemon's record of the rate its engine reports for
// itself: the generation-speed figures parsed out of the engine's captured
// output lines, and the last one the sampler folded into the history. It
// answers two questions with two clocks:
//
//   - the sampler tick asks what the interval saw: a figure arrived, the tick
//     gets a point; none arrived, the tick gets nothing — a point is an
//     observation, and holding the last figure onto a quiet tick would draw
//     a generation rate for a window that generated nothing;
//   - a metrics read asks what stands now: the last figure counts only while
//     it is no older than a sampling interval, so an engine that has gone
//     quiet lets the current figure lapse rather than standing as the rate
//     of some earlier burst.
type engineSpeed struct {
	mu       sync.Mutex
	value    float64
	observed time.Time // engine clock of the last figure
	have     bool
	lastTick time.Time // sampler clock of the last tick that was asked
}

// observe records one parsed figure against the daemon's clock.
func (s *engineSpeed) observe(v float64, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.value, s.observed, s.have = v, now, true
}

// sampleTick is the sampler's fold: the point for this tick, or nil when
// the interval since the last tick saw no figure. Every tick updates the
// baseline, whether or not it produced a point — the question the next tick
// asks is "since this one".
func (s *engineSpeed) sampleTick(now time.Time) *float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	var point *float64
	if s.have && s.observed.After(s.lastTick) {
		v := s.value
		point = &v
	}
	s.lastTick = now
	return point
}

// current is the figure a metrics read reports: the last one seen, while a
// tick's worth of quiet has not aged it out.
func (s *engineSpeed) current(now time.Time, staleAfter time.Duration) *float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.have || now.Sub(s.observed) > staleAfter {
		return nil
	}
	v := s.value
	return &v
}

// forget drops the record, so one engine's rate is not reported against the
// next — beside the counter baseline's drop and the history's clear, for
// exactly their reason.
func (s *engineSpeed) forget() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.value, s.observed, s.have, s.lastTick = 0, time.Time{}, false, time.Time{}
}

// ObserveEngineLine is the supervisor's output hook: the one place engine
// bytes become a speed figure. It is cheap and never blocks — a regex over a
// log line — and lines that carry no figure are most lines, so they cost the
// scan and nothing else. The CLI wires it to Supervisor.OnOutput on every
// path that starts an engine, including the foreground serve path that
// starts through the supervisor directly rather than through StartEngine.
func (d *Daemon) ObserveEngineLine(line string) {
	if v, ok := metrics.ParseGenerationSpeed(line); ok {
		d.speed.observe(v, d.now())
	}
}

// speedStaleAfter is how long a current speed figure stands without a new
// one: one sampling interval, the same clock the tick's fold runs on, so a
// figure lapses for the read exactly when its tick would have folded nothing.
func (d *Daemon) speedStaleAfter() time.Duration {
	if d.SampleInterval > 0 {
		return d.SampleInterval
	}
	return DefaultSampleInterval
}
