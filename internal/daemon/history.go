package daemon

import (
	"context"
	"sync"
	"time"

	"github.com/spinloop-ai/spinloop/internal/metrics"
)

// historyWindow is how far back the retained system readings reach. It is
// the span the bar format draws, so it bounds the buffer whatever the
// sampler's cadence — a faster cadence simply holds more samples inside the
// same window.
const historyWindow = 10 * time.Minute

// historyLimit is the most samples the buffer holds. The time window is the
// rule the drawing follows; this is the guard for the remote relay, where the
// metrics reply crosses SSM and its output truncates at 4KB. At the default
// 15s cadence the limit never bites — 10 minutes holds exactly 40 samples —
// but a sampler running faster than that (the catch-up cadence just after a
// start) would otherwise fill the reply past the truncation.
const historyLimit = 40

// systemHistory is the daemon's retained readings of the host's CPU, memory
// and GPU figures, taken by the sampler while an engine runs. It survives a
// stop — the readings up to the stop answer what the engine was doing until
// it stopped — and is cleared when the next engine starts, so one engine's
// readings are never reported against another.
type systemHistory struct {
	mu      sync.Mutex
	samples []metrics.HistorySample
}

// add records one reading, dropping what has aged out of the window. The
// sample's own time is the reference, so a reading is never kept longer than
// the window past a later one.
func (h *systemHistory) add(s metrics.HistorySample) {
	h.mu.Lock()
	defer h.mu.Unlock()
	cutoff := time.Unix(s.Time, 0).Add(-historyWindow)
	i := 0
	for i < len(h.samples) && time.Unix(h.samples[i].Time, 0).Before(cutoff) {
		i++
	}
	h.samples = append(h.samples[i:], s)
	if len(h.samples) > historyLimit {
		h.samples = h.samples[len(h.samples)-historyLimit:]
	}
}

// snapshot reports the retained readings, oldest first; nil when there are
// none, so the metrics reply omits the field rather than showing an empty
// window for a daemon that has never run an engine.
func (h *systemHistory) snapshot() []metrics.HistorySample {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.samples) == 0 {
		return nil
	}
	out := make([]metrics.HistorySample, len(h.samples))
	copy(out, h.samples)
	return out
}

// clear drops every reading. StartEngine calls it beside the counter
// baseline's drop, for the same reason: the previous engine's figures must
// not be reported against the next one.
func (h *systemHistory) clear() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.samples = nil
}

// systemSampleOnce takes one reading of the host's figures, for the retained
// history. Unlike the counter scrape it needs no scrape target — the figures
// come from host commands, not from the engine — so an engine with no metrics
// endpoint still yields a window to draw. It runs only while an engine is
// running: a stopped engine has no utilization to chart, and the readings
// taken before the stop remain until the next start. A reading that yields no
// figure at all records nothing and reports nothing: a failed sample is a
// non-observation here as in the activity record, and the on-request
// collection keeps its own error reporting.
func (d *Daemon) systemSampleOnce(ctx context.Context) {
	if state, _, _ := d.Sup.Status(); state != StateRunning {
		return
	}
	if d.Collector == nil {
		return
	}
	var stats metrics.Stats
	d.Collector.System(ctx, &stats)
	sample := metrics.HistorySample{Time: d.now().Unix()}
	if stats.CPU != nil {
		v := stats.CPU.Utilization
		sample.CPU = &v
	}
	if stats.Memory != nil && stats.Memory.Total > 0 {
		v := float64(stats.Memory.Used) / float64(stats.Memory.Total) * 100
		sample.Mem = &v
	}
	for _, g := range stats.GPUs {
		hg := metrics.HistoryGPU{Index: g.Index, Util: g.Utilization}
		if g.MemoryTotal > 0 {
			v := float64(g.MemoryUsed) / float64(g.MemoryTotal) * 100
			hg.Mem = &v
		}
		sample.GPUs = append(sample.GPUs, hg)
	}
	if sample.CPU == nil && sample.Mem == nil && len(sample.GPUs) == 0 {
		return
	}
	d.hist.add(sample)
}
