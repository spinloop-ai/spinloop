package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spinloop-ai/spinloop/internal/metrics"
	"github.com/spinloop-ai/spinloop/internal/remote"
)

func ptrPct(v float64) *float64 { return &v }

func TestBarGlyph(t *testing.T) {
	cases := []struct {
		pct  float64
		want rune
	}{
		{0, '▁'}, {12.4, '▁'}, {12.5, '▂'}, {30, '▃'}, {50, '▅'},
		{75, '▇'}, {87.4, '▇'}, {87.5, '█'}, {100, '█'},
	}
	for _, c := range cases {
		if got := barGlyph(c.pct); got != c.want {
			t.Errorf("barGlyph(%v) = %c, want %c", c.pct, got, c.want)
		}
	}
}

func TestPoolMaxKeepsPeaks(t *testing.T) {
	// A pool keeps its highest reading, so the spike a threshold is about
	// survives the downsampling instead of being averaged away.
	got := poolMax([]float64{1, 9, 2, 8, 3, 7}, 3)
	if len(got) != 3 || got[0] != 9 || got[1] != 8 || got[2] != 7 {
		t.Errorf("poolMax = %v, want [9 8 7]", got)
	}
	// A series no wider than the draw is passed through untouched.
	in := []float64{1, 2, 3}
	if got := poolMax(in, 5); len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Errorf("an unwidened series changed: %v", got)
	}
}

func TestRenderSparkline(t *testing.T) {
	var b bytes.Buffer
	renderSparkline(&b, "CPU", []float64{20, 30}, 40)
	// The trailing figure is the latest sample's percentage — the exact value
	// the last glyph approximates — not the series' first.
	want := "  CPU       " + strings.Repeat(" ", 38) + "▂" + ansiGreen + "▃" + ansiReset + " 30%\n"
	if got := b.String(); got != want {
		t.Errorf("sparkline = %q, want %q", got, want)
	}
}

// Only the final glyph takes the state colour, on the gauge's 80/90
// thresholds, whatever the rest of the window did — the window here holds a
// red spike earlier that must stay uncoloured.
func TestRenderSparklineColoursOnlyTheLastPoint(t *testing.T) {
	cases := []struct {
		last float64
		want string
	}{
		{79.9, ansiGreen + "▇" + ansiReset + " 80%\n"},
		{85, ansiYellow + "▇" + ansiReset + " 85%\n"},
		{95, ansiRed + "█" + ansiReset + " 95%\n"},
	}
	for _, c := range cases {
		var b bytes.Buffer
		renderSparkline(&b, "CPU", []float64{5, 95, c.last}, 40)
		if !strings.HasSuffix(b.String(), c.want) {
			t.Errorf("last point %v: %q, want suffix %q", c.last, b.String(), c.want)
		}
		// Every earlier glyph is uncoloured: the escapes appear once, around
		// the final glyph only.
		if n := strings.Count(b.String(), "\033["); n != 2 {
			t.Errorf("last point %v: %d escape sequences, want 2", c.last, n)
		}
	}
}

func TestRenderGauge(t *testing.T) {
	var b bytes.Buffer
	renderGauge(&b, "CPU", 42)
	want := "  CPU       " + ansiGreen + strings.Repeat("█", 10) + ansiReset + strings.Repeat("░", 15) + " 42%\n"
	if got := b.String(); got != want {
		t.Errorf("gauge = %q, want %q", got, want)
	}
}

func TestValidateMetricsFormat(t *testing.T) {
	for _, f := range []string{"bar", "gauge", "table", "json"} {
		if err := validateMetricsFormat(f); err != nil {
			t.Errorf("%q rejected: %v", f, err)
		}
	}
	err := validateMetricsFormat("csv")
	if err == nil || !strings.Contains(err.Error(), `"csv"`) {
		t.Errorf("csv: %v, want the bad value named", err)
	}
}

// A daemon that predates the history — or a series it never reported — draws
// exactly as the old bar format did, byte for byte.
func TestRenderStatBarsFallsBackToGaugeWithoutHistory(t *testing.T) {
	cpu := &metrics.CpuStat{Utilization: 42}
	mem := &metrics.MemoryStat{Total: 1000, Used: 300}
	gpus := []metrics.GpuStat{{Index: 0, Name: "H100", Utilization: 61, MemoryUsed: 80, MemoryTotal: 160}}

	var got, want bytes.Buffer
	renderStatBars(&got, cpu, mem, gpus, nil, barLineW)
	renderStatGauges(&want, cpu, mem, gpus)
	if got.String() != want.String() {
		t.Errorf("no-history bar differs from the old drawing:\ngot:\n%q\nwant:\n%q", got.String(), want.String())
	}
	if !strings.Contains(got.String(), "░") {
		t.Errorf("the fallback drew no gauge: %q", got.String())
	}
}

func TestRenderStatBarsDrawsHistory(t *testing.T) {
	history := []metrics.HistorySample{
		{Time: 1, CPU: ptrPct(10), Mem: ptrPct(20)},
		{Time: 2, CPU: ptrPct(20), Mem: ptrPct(30)},
		{Time: 3, CPU: ptrPct(95), Mem: ptrPct(40)},
	}
	var b bytes.Buffer
	renderStatBars(&b, nil, nil, nil, history, barLineW)
	lines := strings.Split(strings.TrimSuffix(b.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("drew %d lines, want CPU and RAM: %q", len(lines), b.String())
	}
	if !strings.Contains(lines[0], ansiRed+"█"+ansiReset+" 95%") {
		t.Errorf("CPU line lost its spike or its red last point: %q", lines[0])
	}
	if !strings.Contains(lines[1], ansiGreen+"▄"+ansiReset+" 40%") {
		t.Errorf("RAM line: %q", lines[1])
	}
}

// The fallback is per series: a series with retained readings draws them, a
// series without falls back to its current reading on the same screen.
func TestRenderStatBarsFallsBackPerSeries(t *testing.T) {
	history := []metrics.HistorySample{
		{Time: 1, CPU: ptrPct(10)},
		{Time: 2, CPU: ptrPct(20)},
	}
	var b bytes.Buffer
	renderStatBars(&b, nil, &metrics.MemoryStat{Total: 1000, Used: 300}, nil, history, barLineW)
	lines := strings.Split(strings.TrimSuffix(b.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("drew %d lines, want 2: %q", len(lines), b.String())
	}
	if strings.Contains(lines[0], "░") || !strings.Contains(lines[0], " 20%") {
		t.Errorf("CPU did not draw its history: %q", lines[0])
	}
	if !strings.Contains(lines[1], "░") || !strings.Contains(lines[1], " 30%") {
		t.Errorf("RAM did not fall back to the gauge: %q", lines[1])
	}
}

// A stopped engine carries no current figures, so the bar format draws the
// retained readings alone — including the GPU series the engine ran on, which
// the current reading no longer names.
func TestRenderStatBarsStoppedEngineDrawsHistoryAlone(t *testing.T) {
	history := []metrics.HistorySample{
		{Time: 1, CPU: ptrPct(10), GPUs: []metrics.HistoryGPU{{Index: 0, Util: 50, Mem: ptrPct(50)}}},
		{Time: 2, CPU: ptrPct(20), GPUs: []metrics.HistoryGPU{{Index: 0, Util: 60, Mem: ptrPct(60)}}},
	}
	var b bytes.Buffer
	renderStatBars(&b, nil, nil, nil, history, barLineW)
	lines := strings.Split(strings.TrimSuffix(b.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("drew %d lines, want CPU, GPU util and GPU mem: %q", len(lines), b.String())
	}
	if !strings.HasPrefix(lines[0], "  CPU       ") {
		t.Errorf("first line: %q", lines[0])
	}
	// One GPU: the plain "GPU" prefix, as the gauge used it.
	if !strings.Contains(lines[1], "GPU util") || !strings.Contains(lines[1], " 60%") {
		t.Errorf("GPU util line: %q", lines[1])
	}
	if !strings.Contains(lines[2], "GPU mem") || !strings.Contains(lines[2], " 60%") {
		t.Errorf("GPU mem line: %q", lines[2])
	}

	// Two GPUs: the index goes in the label.
	history = []metrics.HistorySample{
		{Time: 1, CPU: ptrPct(10), GPUs: []metrics.HistoryGPU{
			{Index: 0, Util: 10, Mem: ptrPct(10)},
			{Index: 1, Util: 90, Mem: ptrPct(20)},
		}},
	}
	b.Reset()
	renderStatBars(&b, nil, nil, nil, history, barLineW)
	out := b.String()
	if !strings.Contains(out, "GPU 0 util") || !strings.Contains(out, "GPU 1 util") ||
		!strings.Contains(out, "GPU 0 mem") || !strings.Contains(out, "GPU 1 mem") {
		t.Errorf("two GPUs: %q", out)
	}
}

// The gauge format draws the current reading only, whatever the history
// holds — and a stopped engine's reading carries nothing, so it draws none.
func TestRenderStatGaugesIgnoresHistory(t *testing.T) {
	cpu := &metrics.CpuStat{Utilization: 50}
	var b bytes.Buffer
	renderStatGauges(&b, cpu, nil, nil)
	if !strings.Contains(b.String(), " 50%") || strings.Contains(b.String(), "▁") {
		t.Errorf("gauge drew history or the wrong value: %q", b.String())
	}
	b.Reset()
	renderStatGauges(&b, nil, nil, nil)
	if b.String() != "" {
		t.Errorf("a stopped engine drew gauges: %q", b.String())
	}
}

func TestFormatMetricsBarStoppedWithHistory(t *testing.T) {
	resp := &remote.StatsResponse{
		Environment: "prod", State: "stopped", ModelID: "org/qwen:q4",
		LastActiveAt: "2026-08-21T10:00:00Z", IdleSeconds: 12,
		History: []metrics.HistorySample{
			{Time: 1, CPU: ptrPct(10)},
			{Time: 2, CPU: ptrPct(20)},
		},
	}
	var b bytes.Buffer
	if err := formatMetricsBar(resp, remote.Config{}, &b); err != nil {
		t.Fatal(err)
	}
	want := "prod  stopped  org/qwen:q4\n" +
		"  last active 12s ago\n" +
		"  CPU       " + strings.Repeat(" ", 38) + "▁" + ansiGreen + "▂" + ansiReset + " 20%\n"
	if got := b.String(); got != want {
		t.Errorf("stopped bar = %q, want %q", got, want)
	}

	// The gauge format draws no series for a stopped endpoint: the header and
	// the last-active line, and nothing after.
	b.Reset()
	if err := formatMetricsGauge(resp, remote.Config{}, &b); err != nil {
		t.Fatal(err)
	}
	want = "prod  stopped  org/qwen:q4\n" +
		"  last active 12s ago\n"
	if got := b.String(); got != want {
		t.Errorf("stopped gauge = %q, want %q", got, want)
	}
}

func TestFormatMetricsBarRunning(t *testing.T) {
	resp := &remote.StatsResponse{
		Environment: "prod", State: "running", InstanceType: "g5.xlarge",
		ModelID: "org/qwen:q4", Version: "0.4.3",
		LastActiveAt: "2026-08-21T10:00:00Z", IdleSeconds: 3,
		CPU:    &metrics.CpuStat{Utilization: 62},
		Memory: &metrics.MemoryStat{Total: 1000, Used: 300},
		GPUs:   []metrics.GpuStat{{Index: 0, Name: "H100", Utilization: 61, MemoryUsed: 80, MemoryTotal: 160}},
		Tokens: &remote.TokenStats{Running: 2, PromptTokens: 4096, GenerationTokens: 1024, Requests: 17},
		History: []metrics.HistorySample{
			{Time: 1, CPU: ptrPct(10), Mem: ptrPct(20), GPUs: []metrics.HistoryGPU{{Index: 0, Util: 50, Mem: ptrPct(50)}}},
			{Time: 2, CPU: ptrPct(20), Mem: ptrPct(30), GPUs: []metrics.HistoryGPU{{Index: 0, Util: 61, Mem: ptrPct(50)}}},
		},
	}
	var b bytes.Buffer
	if err := formatMetricsBar(resp, remote.Config{}, &b); err != nil {
		t.Fatal(err)
	}
	got := b.String()
	if !strings.HasPrefix(got, "prod  running  g5.xlarge  org/qwen:q4  0.4.3\n") {
		t.Errorf("header: %q", got)
	}
	if !strings.Contains(got, "  last active 3s ago\n") {
		t.Errorf("last active line missing: %q", got)
	}
	// Every series drew a sparkline from the history: no gauge in the output.
	if strings.Contains(got, "░") {
		t.Errorf("a series fell back to the gauge although history holds it: %q", got)
	}
	for _, label := range []string{"CPU", "RAM", "GPU util", "GPU mem"} {
		if !strings.Contains(got, label) {
			t.Errorf("series %q missing: %q", label, got)
		}
	}
	if !strings.Contains(got, "  running:          2\n") || !strings.Contains(got, "  requests:         17\n") {
		t.Errorf("token block missing: %q", got)
	}
}

func TestFormatMetricsJSONCarriesHistory(t *testing.T) {
	resp := &remote.StatsResponse{
		Environment: "prod", State: "running",
		CPU: &metrics.CpuStat{Utilization: 62},
		History: []metrics.HistorySample{
			{Time: 1786276800, CPU: ptrPct(62), GPUs: []metrics.HistoryGPU{{Index: 0, Util: 61, Mem: ptrPct(50)}}},
		},
	}
	var b bytes.Buffer
	if err := formatMetricsJSON(resp, false, remote.Config{}, &b); err != nil {
		t.Fatal(err)
	}
	got := b.String()
	for _, want := range []string{`"history"`, `"t":`, `"c":`, `"g":`, `"i":`, `"u":`} {
		if !strings.Contains(got, want) {
			t.Errorf("json %s: missing %s", want, got)
		}
	}
	// Absent history stays absent, as on the daemon.
	b.Reset()
	resp.History = nil
	if err := formatMetricsJSON(resp, false, remote.Config{}, &b); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(b.String(), "history") {
		t.Errorf("absent history serialised: %s", b.String())
	}
}
