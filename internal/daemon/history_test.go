//go:build !windows

package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spinloop-ai/spinloop/internal/metrics"
	"github.com/spinloop-ai/spinloop/internal/remote"
)

func f64(v float64) *float64 { return &v }

// linuxCollector is a Collector stub reading a Linux host with two GPUs: the
// vmstat, free and nvidia-smi outputs in the shapes the parsers know. The
// expected figures: CPU 30%, memory 13.01% used, GPU 0 at 12% utilisation and
// 17.78% memory, GPU 1 at 97% and 88.89%.
func linuxCollector() *metrics.Collector {
	return &metrics.Collector{
		GOOS: "linux",
		Run: func(ctx context.Context, name string, args ...string) (string, error) {
			switch name {
			case "vmstat":
				// The parser reads the last line's columns: r b swpd free buff
				// cache si so bi bo in cs us sy id wa st — id at column 14.
				return "procs -----------memory---------- ---swap-- -----io---- -system-- ------cpu-----\n" +
					" r  b   swpd   free   buff  cache   si   so    bi    bo   in   cs us sy id wa st\n" +
					" 1  0      0 947184  84224 590452    0    0    31    17  210  350  3  1 95  1  0\n" +
					" 2  0      0 947184  84224 590452    0    0     0     0  180  300 20  5 70  5  0\n", nil
			case "free":
				return "              total        used        free\n" +
					"Mem:    33020416512  4294967296 12884901888\n", nil
			case "nvidia-smi":
				return "0, NVIDIA L40S, 12, 8192, 46080, 42\n" +
					"1, NVIDIA L40S, 97, 40960, 46080, 71\n", nil
			}
			return "", errors.New("unexpected command " + name)
		},
	}
}

func almostEqual(a, b float64) bool { return math.Abs(a-b) < 0.01 }

func TestSystemHistoryWindowAndLimit(t *testing.T) {
	var h systemHistory
	if got := h.snapshot(); got != nil {
		t.Fatalf("a fresh buffer snapshots %d samples, want none", len(got))
	}

	add := func(at time.Time) { h.add(metrics.HistorySample{Time: at.Unix(), CPU: f64(10)}) }
	add(baseTime)
	add(baseTime.Add(15 * time.Second))
	if got := h.snapshot(); len(got) != 2 || got[0].Time != baseTime.Unix() ||
		got[1].Time != baseTime.Add(15*time.Second).Unix() {
		t.Fatalf("snapshot = %+v, want oldest first", got)
	}

	// What has aged out of the window is dropped by the next append, not by
	// the snapshot: retention is a property of the buffer, not of a read.
	add(baseTime.Add(11 * time.Minute))
	if got := h.snapshot(); len(got) != 1 ||
		got[0].Time != baseTime.Add(11*time.Minute).Unix() {
		t.Errorf("after a jump past the window: %+v, want only the newest", got)
	}

	// The limit holds the buffer however fast the sampler runs, keeping the
	// newest samples.
	h.clear()
	for i := 0; i < historyLimit+10; i++ {
		h.add(metrics.HistorySample{Time: baseTime.Add(time.Duration(i) * time.Second).Unix(), CPU: f64(10)})
	}
	got := h.snapshot()
	if len(got) != historyLimit {
		t.Fatalf("a fast sampler left %d samples, want the limit of %d", len(got), historyLimit)
	}
	if got[len(got)-1].Time != baseTime.Add(time.Duration(historyLimit+9)*time.Second).Unix() {
		t.Errorf("the limit dropped the newest sample: last = %+v", got[len(got)-1])
	}

	// A snapshot is a copy: a held reply does not move under later appends.
	held := h.snapshot()
	h.add(metrics.HistorySample{Time: baseTime.Add(2 * time.Hour).Unix(), CPU: f64(10)})
	if len(held) != historyLimit {
		t.Errorf("a held snapshot changed from %d to %d samples", historyLimit, len(held))
	}

	h.clear()
	if got := h.snapshot(); got != nil {
		t.Errorf("clear left %d samples", len(got))
	}
}

func TestSystemSampleOnce(t *testing.T) {
	d := testDaemon(t, `trap 'exit 0' TERM
while true; do sleep 0.05; done`)
	d.Now = func() time.Time { return baseTime }
	d.Collector = linuxCollector()

	// Nothing running: no reading is taken, however many times it is asked.
	d.systemSampleOnce(context.Background())
	if got := d.hist.snapshot(); len(got) != 0 {
		t.Fatalf("a reading was taken with no engine running: %+v", got)
	}

	if err := d.Push(remote.DeployConfig{Runner: "llamacpp", ModelID: "m"}); err != nil {
		t.Fatal(err)
	}
	if err := d.StartEngine(); err != nil {
		t.Fatal(err)
	}
	defer d.Sup.Stop()
	waitForState(t, d.Sup, StateRunning)

	d.systemSampleOnce(context.Background())
	got := d.hist.snapshot()
	if len(got) != 1 {
		t.Fatalf("after one tick: %d samples, want 1", len(got))
	}
	s := got[0]
	if s.Time != baseTime.Unix() {
		t.Errorf("sample time = %d, want %d", s.Time, baseTime.Unix())
	}
	if s.CPU == nil || !almostEqual(*s.CPU, 30) {
		t.Errorf("sample cpu = %v, want 30", s.CPU)
	}
	if s.Mem == nil || !almostEqual(*s.Mem, 13.008) {
		t.Errorf("sample mem = %v, want ~13.01", s.Mem)
	}
	if len(s.GPUs) != 2 {
		t.Fatalf("sample gpus = %+v, want two", s.GPUs)
	}
	if s.GPUs[0].Index != 0 || s.GPUs[0].Util != 12 || s.GPUs[0].Mem == nil ||
		!almostEqual(*s.GPUs[0].Mem, 17.778) {
		t.Errorf("gpu 0 = %+v", s.GPUs[0])
	}
	if s.GPUs[1].Index != 1 || s.GPUs[1].Util != 97 || s.GPUs[1].Mem == nil ||
		!almostEqual(*s.GPUs[1].Mem, 88.889) {
		t.Errorf("gpu 1 = %+v", s.GPUs[1])
	}

	// A second tick at a later time appends, oldest first.
	d.Now = func() time.Time { return baseTime.Add(15 * time.Second) }
	d.systemSampleOnce(context.Background())
	if got := d.hist.snapshot(); len(got) != 2 || got[1].Time != baseTime.Add(15*time.Second).Unix() {
		t.Errorf("after a second tick: %+v, want the reading appended", got)
	}
}

// A failed reading is a non-observation: nothing is recorded and nothing is
// reported, because the on-request collection keeps its own error reporting
// and a transient sampling failure is not a condition worth surfacing per tick.
func TestSystemSampleOnceRecordsNothingOnFailure(t *testing.T) {
	d := testDaemon(t, `trap 'exit 0' TERM
while true; do sleep 0.05; done`)
	d.Now = func() time.Time { return baseTime }
	// Every host command is missing: the absent-source case.
	d.Collector = &metrics.Collector{
		GOOS: "linux",
		Run:  func(ctx context.Context, name string, args ...string) (string, error) { return "", exec.ErrNotFound },
	}
	if err := d.Push(remote.DeployConfig{Runner: "llamacpp", ModelID: "m"}); err != nil {
		t.Fatal(err)
	}
	if err := d.StartEngine(); err != nil {
		t.Fatal(err)
	}
	defer d.Sup.Stop()
	waitForState(t, d.Sup, StateRunning)

	d.systemSampleOnce(context.Background())
	if got := d.hist.snapshot(); len(got) != 0 {
		t.Errorf("a failed reading recorded a sample: %+v", got)
	}
	// And a reading that fails part-way — the GPU source there, the rest
	// absent — records what it has rather than all or nothing.
	d.Collector = &metrics.Collector{
		GOOS: "linux",
		Run: func(ctx context.Context, name string, args ...string) (string, error) {
			if name == "nvidia-smi" {
				return "", exec.ErrNotFound
			}
			return linuxCollector().Run(ctx, name, args...)
		},
	}
	d.systemSampleOnce(context.Background())
	got := d.hist.snapshot()
	if len(got) != 1 || got[0].CPU == nil || got[0].Mem == nil || len(got[0].GPUs) != 0 {
		t.Errorf("a partial reading was not recorded as partial: %+v", got)
	}
}

func TestSystemHistoryClearsOnStartAndSurvivesAStop(t *testing.T) {
	d := testDaemon(t, `trap 'exit 0' TERM
while true; do sleep 0.05; done`)
	now := baseTime
	d.Now = func() time.Time { return now }
	d.Collector = linuxCollector()
	if err := d.Push(remote.DeployConfig{Runner: "llamacpp", ModelID: "m"}); err != nil {
		t.Fatal(err)
	}

	// A reading taken before the start must not be reported against the
	// engine that starts: the clear goes through StartEngine, not the test.
	d.hist.add(metrics.HistorySample{Time: now.Add(-time.Hour).Unix(), CPU: f64(99)})
	if err := d.StartEngine(); err != nil {
		t.Fatal(err)
	}
	defer d.Sup.Stop()
	waitForState(t, d.Sup, StateRunning)
	if got := d.hist.snapshot(); len(got) != 0 {
		t.Fatalf("the previous engine's reading survived the start: %+v", got)
	}

	now = now.Add(2 * time.Minute)
	d.systemSampleOnce(context.Background())
	if got := d.hist.snapshot(); len(got) != 1 {
		t.Fatalf("after a tick: %d samples, want 1", len(got))
	}

	// The stop does not clear: the readings up to the stop say what the engine
	// was doing until it stopped.
	if err := d.Sup.Stop(); err != nil {
		t.Fatal(err)
	}
	waitForState(t, d.Sup, StateStopped)
	if got := d.hist.snapshot(); len(got) != 1 || got[0].Time != now.Unix() {
		t.Errorf("a stop cleared the history: %+v", got)
	}

	// The next engine clears it.
	if err := d.StartEngine(); err != nil {
		t.Fatal(err)
	}
	waitForState(t, d.Sup, StateRunning)
	if got := d.hist.snapshot(); len(got) != 0 {
		t.Errorf("the first engine's readings survived into the second: %+v", got)
	}
}

// TestMetricsExposesHistory covers what /v1/metrics says: the retained
// readings alongside the current ones, absent where none has been taken, and
// surviving a stop while the running-engine figures go.
func TestMetricsExposesHistory(t *testing.T) {
	d := testDaemon(t, `trap 'exit 0' TERM
while true; do sleep 0.05; done`)
	d.Now = func() time.Time { return baseTime }
	d.Collector = linuxCollector()

	// A daemon that has never run an engine says so by omission, not by an
	// empty window.
	stats := d.Metrics(context.Background())
	if stats.History != nil {
		t.Errorf("a daemon that has served nothing reported history: %+v", stats.History)
	}
	if body, _ := json.Marshal(stats); bytes.Contains(body, []byte("history")) {
		t.Errorf("absent history still serialised: %s", body)
	}

	if err := d.Push(remote.DeployConfig{Runner: "llamacpp", ModelID: "m"}); err != nil {
		t.Fatal(err)
	}
	if err := d.StartEngine(); err != nil {
		t.Fatal(err)
	}
	defer d.Sup.Stop()
	waitForState(t, d.Sup, StateRunning)

	d.systemSampleOnce(context.Background())
	stats = d.Metrics(context.Background())
	if len(stats.History) != 1 || stats.History[0].CPU == nil || !almostEqual(*stats.History[0].CPU, 30) {
		t.Fatalf("metrics carried no usable history: %+v", stats.History)
	}

	// The reply the control plane actually reads carries the compact field
	// names, one letter each.
	if body, _ := json.Marshal(stats); !strings.Contains(string(body), `"t":`) ||
		!strings.Contains(string(body), `"g":`) {
		t.Errorf("the history did not serialise with its one-letter fields: %s", body)
	}

	// Stopping drops the running-engine figures and keeps the history.
	if err := d.Sup.Stop(); err != nil {
		t.Fatal(err)
	}
	waitForState(t, d.Sup, StateStopped)
	stats = d.Metrics(context.Background())
	if stats.CPU != nil || stats.Memory != nil || len(stats.GPUs) > 0 || stats.Tokens != nil {
		t.Errorf("a stopped engine reported running-engine figures: %+v", stats)
	}
	if len(stats.History) != 1 {
		t.Errorf("a stopped engine lost its history: %+v", stats.History)
	}
}

// The loop is what the deployment runs: while an engine runs, one system
// reading per tick, with no scrape target involved — and the readings stop
// when the engine stops, while the retention ends only at the next start.
func TestSamplerTakesSystemReadingsEachTick(t *testing.T) {
	d := testDaemon(t, `trap 'exit 0' TERM
while true; do sleep 0.05; done`)
	clock := &fakeClock{t: baseTime}
	d.Now = clock.now
	d.Collector = linuxCollector()
	// No scrape target is set: the system readings must not depend on one.
	// The cadence comes from the tick interval rather than the catch-up,
	// which without a scrape target does not apply — there are no counters
	// coming, so there is nothing to catch up to.
	d.SampleInterval = 5 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		d.SampleActivity(ctx)
		close(done)
	}()
	defer func() {
		cancel()
		<-done
	}()

	if err := d.Push(remote.DeployConfig{Runner: "llamacpp", ModelID: "m"}); err != nil {
		t.Fatal(err)
	}
	if err := d.StartEngine(); err != nil {
		t.Fatal(err)
	}
	defer d.Sup.Stop()
	waitForState(t, d.Sup, StateRunning)

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if len(d.hist.snapshot()) >= 2 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got := len(d.hist.snapshot()); got < 2 {
		t.Fatalf("the sampler took %d system readings in 3s, want at least 2", got)
	}

	// The stop ends the sampling but not the retention.
	if err := d.Sup.Stop(); err != nil {
		t.Fatal(err)
	}
	waitForState(t, d.Sup, StateStopped)
	n := len(d.hist.snapshot())
	time.Sleep(50 * time.Millisecond)
	if got := len(d.hist.snapshot()); got != n {
		t.Errorf("a stopped engine kept accumulating readings: %d -> %d", n, got)
	}
}

// The whole point of the sampler: a metrics request runs no host command, so
// a handler cannot be held up by a slow one. `top -l 1` on a loaded macOS host
// takes seconds, which is what made a polled node render as unreachable while
// it was answering fine.
func TestMetricsRunsNoHostCommands(t *testing.T) {
	d := testDaemon(t, `trap 'exit 0' TERM
while true; do sleep 0.05; done`)
	var during int32
	base := linuxCollector()
	inner := base.Run
	base.Run = func(ctx context.Context, name string, args ...string) (string, error) {
		atomic.AddInt32(&during, 1)
		return inner(ctx, name, args...)
	}
	d.Collector = base
	if err := d.Push(remote.DeployConfig{Runner: "llamacpp", ModelID: "m"}); err != nil {
		t.Fatal(err)
	}
	if err := d.StartEngine(); err != nil {
		t.Fatal(err)
	}
	defer d.Sup.Stop()
	waitForState(t, d.Sup, StateRunning)

	d.systemSampleOnce(context.Background())
	sampled := atomic.LoadInt32(&during)
	if sampled == 0 {
		t.Fatal("the sampler ran no host commands; it is what collects them")
	}
	for i := 0; i < 5; i++ {
		if stats := d.Metrics(context.Background()); stats.Memory == nil {
			t.Fatalf("request %d reported no memory figure from the last sample", i)
		}
	}
	if got := atomic.LoadInt32(&during); got != sampled {
		t.Errorf("metrics requests ran %d host commands, want none", got-sampled)
	}
}

// A collection failure is reported to the caller. The handler no longer
// collects, so the sample is the only thing that can carry the error — losing
// it would turn a broken source into figures that are silently absent.
func TestSampledCollectionErrorsReachMetrics(t *testing.T) {
	d := testDaemon(t, `trap 'exit 0' TERM
while true; do sleep 0.05; done`)
	d.Collector = &metrics.Collector{
		GOOS: "linux",
		Run: func(ctx context.Context, name string, args ...string) (string, error) {
			return "", errors.New("vmstat exploded")
		},
	}
	if err := d.Push(remote.DeployConfig{Runner: "llamacpp", ModelID: "m"}); err != nil {
		t.Fatal(err)
	}
	if err := d.StartEngine(); err != nil {
		t.Fatal(err)
	}
	defer d.Sup.Stop()
	waitForState(t, d.Sup, StateRunning)

	d.systemSampleOnce(context.Background())
	stats := d.Metrics(context.Background())
	var found bool
	for _, e := range stats.Errors {
		if strings.Contains(e, "vmstat exploded") {
			found = true
		}
	}
	if !found {
		t.Errorf("the collection failure did not reach the caller: %v", stats.Errors)
	}
	// Reported once, from the one collection, not once per reader.
	if n := len(stats.Errors); n != len(d.Metrics(context.Background()).Errors) {
		t.Errorf("errors accumulate across requests: %d", n)
	}
}

// The catch-up interval is for a reading that is actually coming. An engine
// whose runner exposes no metrics endpoint yields no counters ever, so the
// loop must settle at its tick rather than spin — it runs the host commands
// on every pass.
func TestNoScrapeTargetLeavesTheCatchUpInterval(t *testing.T) {
	d := testDaemon(t, `trap 'exit 0' TERM
while true; do sleep 0.05; done`)
	if d.awaitingFirstSample() {
		t.Error("no scrape target: the catch-up interval should not apply")
	}
	// An address with no dialect is the readiness-probe-only case: still
	// nothing to scrape, so still no catching up to do.
	d.SetScrape(metrics.ScrapeTarget{BaseURL: "http://127.0.0.1:8080"})
	if d.awaitingFirstSample() {
		t.Error("an address with no metrics dialect should not hold the catch-up")
	}
	d.SetScrape(metrics.ScrapeTarget{BaseURL: "http://127.0.0.1:8080", Engine: "llamacpp"})
	if !d.awaitingFirstSample() {
		t.Error("a scrape target with no reading yet should hold the catch-up")
	}
}
