package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/spinloop-ai/spinloop/internal/cloud"
	"github.com/spinloop-ai/spinloop/internal/fleet"
	"github.com/spinloop-ai/spinloop/internal/metrics"
)

func ptrPct(v float64) *float64 { return &v }

func ptrInt(v int) *int { return &v }

func TestBarGlyph(t *testing.T) {
	cases := []struct {
		pct  float64
		want rune
	}{
		{0, '▁'}, {12.4, '▁'}, {12.5, '▁'}, {30, '▃'}, {50, '▄'},
		{75, '▆'}, {87.4, '▇'}, {87.5, '▇'}, {100, '▇'},
	}
	for _, c := range cases {
		if got := barGlyph(c.pct); got != c.want {
			t.Errorf("barGlyph(%v) = %c, want %c", c.pct, got, c.want)
		}
	}
}

// The sparkline never draws a full block: the highest value caps at the
// seven-eighths glyph, so a maxed row leaves a sliver of space above it and
// adjacent rows read as separate bars.
func TestBarGlyphNeverFullBlock(t *testing.T) {
	for pct := 0.0; pct <= 100.0; pct += 0.5 {
		if g := barGlyph(pct); g == '█' {
			t.Fatalf("barGlyph(%v) = full block, want seven-eighths or less", pct)
		}
	}
	if g := barGlyph(100); g != '▇' {
		t.Errorf("barGlyph(100) = %c, want ▇", g)
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
	// An uneven split pools the same values, one column each.
	got = poolMax([]float64{1, 5, 2, 9, 3, 4, 8}, 3)
	if len(got) != 3 || got[0] != 5 || got[1] != 9 || got[2] != 8 {
		t.Errorf("uneven pool = %v, want [5 9 8]", got)
	}
	// The newest sample lands in the final column: at the tile's width a
	// 29-sample window splits unevenly, and the split's rounding leaves the
	// last reading out of that column — and the trailing figure is the last
	// reading's value, so it must be in the pool. The values rise, so the
	// last column's maximum is its newest sample only if the sample is there.
	vals := make([]float64, 29)
	for i := range vals {
		vals[i] = float64(i + 1)
	}
	if got := poolMax(vals, 25); got[24] != 29 {
		t.Errorf("the newest sample dropped out of the last pool: %v, want 29", got[24])
	}
}

func TestRenderSparkline(t *testing.T) {
	var b bytes.Buffer
	renderSparkline(&b, "CPU", []float64{20, 30}, 40, 7)
	// The trailing figure is the latest sample's percentage — the exact value
	// the last glyph approximates — not the series' first. Both points sit
	// below the 80% threshold, so neither takes a colour.
	want := "  CPU     " + strings.Repeat(" ", 38) + "▂▃" + " 30%\n"
	if got := b.String(); got != want {
		t.Errorf("sparkline = %q, want %q", got, want)
	}
}

// Every point takes the state colour of its own value, on the gauge's 80/90
// thresholds, whatever the rest of the window did — the window here holds a
// red spike and a yellow reading earlier, and they keep their colour where
// they sit while the calm points around them do not.
func TestRenderSparklineColoursEveryPoint(t *testing.T) {
	var b bytes.Buffer
	renderSparkline(&b, "CPU", []float64{5, 95, 85, 30}, 40, 7)
	want := "  CPU     " + strings.Repeat(" ", 36) +
		"▁" + ansiRed + "▇" + ansiReset +
		ansiYellow + "▆" + ansiReset +
		"▃" + " 30%\n"
	if got := b.String(); got != want {
		t.Errorf("sparkline = %q, want %q", got, want)
	}
}

// The latest point takes the colour of its own value, like any other point,
// on the gauge's thresholds with their boundaries: 80 itself is yellow, 90
// itself still is, and only above 90 turns red.
func TestRenderSparklineColoursTheLastPointByItsValue(t *testing.T) {
	cases := []struct {
		last float64
		want string
	}{
		{79.9, "▆" + " 80%\n"},
		{80, ansiYellow + "▆" + ansiReset + " 80%\n"},
		{85, ansiYellow + "▆" + ansiReset + " 85%\n"},
		{90, ansiYellow + "▇" + ansiReset + " 90%\n"},
		{95, ansiRed + "▇" + ansiReset + " 95%\n"},
	}
	for _, c := range cases {
		var b bytes.Buffer
		renderSparkline(&b, "CPU", []float64{5, 95, c.last}, 40, 7)
		if !strings.HasSuffix(b.String(), c.want) {
			t.Errorf("last point %v: %q, want suffix %q", c.last, b.String(), c.want)
		}
	}
}

func TestRenderGauge(t *testing.T) {
	var b bytes.Buffer
	renderGauge(&b, "CPU", 42, gaugeW, 7)
	want := "  CPU     " + ansiGreen + strings.Repeat("█", 10) + ansiReset + strings.Repeat("░", 15) + " 42%\n"
	if got := b.String(); got != want {
		t.Errorf("gauge = %q, want %q", got, want)
	}
	// A value beyond 100 fills the gauge rather than spilling past it.
	b.Reset()
	renderGauge(&b, "CPU", 150, gaugeW, 7)
	want = "  CPU     " + ansiRed + strings.Repeat("█", 25) + ansiReset + " 150%\n"
	if got := b.String(); got != want {
		t.Errorf("out-of-range gauge = %q, want %q", got, want)
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
	renderStatBars(&got, cpu, mem, gpus, nil, nil, barLineW)
	renderStatGauges(&want, cpu, mem, gpus, nil, nil)
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
	renderStatBars(&b, nil, nil, nil, nil, history, barLineW)
	lines := strings.Split(strings.TrimSuffix(b.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("drew %d lines, want CPU and RAM: %q", len(lines), b.String())
	}
	// The spike is the latest reading, so it is the last point and red; the
	// earlier, calm points carry no colour.
	if !strings.Contains(lines[0], ansiRed+"▇"+ansiReset+" 95%") {
		t.Errorf("CPU line lost its red spike: %q", lines[0])
	}
	if strings.Contains(lines[0], ansiGreen) {
		t.Errorf("CPU line coloured a below-threshold point green: %q", lines[0])
	}
	// The RAM readings sit below the thresholds, so none takes a colour.
	if !strings.Contains(lines[1], "▃ 40%") {
		t.Errorf("RAM line: %q", lines[1])
	}
	if strings.Contains(lines[1], "\033[") {
		t.Errorf("RAM line coloured a below-threshold point: %q", lines[1])
	}
}

// Two adjacent series both pinned at 100% draw at seven-eighths, so neither
// row reaches the top of its cell: the rows read as separate bars rather than
// one solid block. The full block must not appear anywhere in the render.
func TestRenderStatBarsMaxedRowsStopShortOfFull(t *testing.T) {
	history := []metrics.HistorySample{
		{Time: 1, CPU: ptrPct(100), Mem: ptrPct(100)},
		{Time: 2, CPU: ptrPct(100), Mem: ptrPct(100)},
	}
	var b bytes.Buffer
	renderStatBars(&b, nil, nil, nil, nil, history, barLineW)
	out := b.String()
	if strings.Contains(out, "█") {
		t.Errorf("a maxed series drew a full block:\n%s", out)
	}
	if !strings.Contains(out, "▇") {
		t.Errorf("a maxed series did not draw the seven-eighths block:\n%s", out)
	}
	if !strings.Contains(out, " 100%") {
		t.Errorf("a maxed series lost its trailing figure:\n%s", out)
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
	renderStatBars(&b, nil, &metrics.MemoryStat{Total: 1000, Used: 300}, nil, nil, history, barLineW)
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
	renderStatBars(&b, nil, nil, nil, nil, history, barLineW)
	lines := strings.Split(strings.TrimSuffix(b.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("drew %d lines, want CPU, GPU and GPU mem: %q", len(lines), b.String())
	}
	if !strings.HasPrefix(lines[0], "  CPU     ") {
		t.Errorf("first line: %q", lines[0])
	}
	// One GPU: the plain "GPU" prefix, as the gauge used it.
	if !strings.Contains(lines[1], "GPU") || !strings.Contains(lines[1], " 60%") {
		t.Errorf("GPU line: %q", lines[1])
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
	renderStatBars(&b, nil, nil, nil, nil, history, barLineW)
	out := b.String()
	if !strings.Contains(out, "GPU 0") || !strings.Contains(out, "GPU 1") ||
		!strings.Contains(out, "GPU 0 mem") || !strings.Contains(out, "GPU 1 mem") {
		t.Errorf("two GPUs: %q", out)
	}
}

// A memory reading with no total reports 0, not a division by it.
func TestBarSeriesListMemoryWithoutTotal(t *testing.T) {
	var b bytes.Buffer
	renderStatBars(&b, nil, &metrics.MemoryStat{Total: 0, Used: 100}, nil, nil, nil, barLineW)
	if !strings.Contains(b.String(), " 0%") || strings.Contains(b.String(), "NaN") {
		t.Errorf("a memory reading with no total: %q", b.String())
	}
}

// A GPU that appears in only some samples: each of its series draws the
// samples that carry it, and the pick yields nothing for the rest.
func TestRenderStatBarsGPUInOnlySomeSamples(t *testing.T) {
	history := []metrics.HistorySample{
		{Time: 1, GPUs: []metrics.HistoryGPU{{Index: 0, Util: 10, Mem: ptrPct(20)}}},
		{Time: 2, GPUs: []metrics.HistoryGPU{{Index: 1, Util: 90, Mem: ptrPct(80)}}},
	}
	var b bytes.Buffer
	renderStatBars(&b, nil, nil, nil, nil, history, barLineW)
	out := b.String()
	// Both GPUs are named in the union, so all four series draw.
	for _, want := range []string{"GPU 0", "GPU 1", "GPU 0 mem", "GPU 1 mem"} {
		if !strings.Contains(out, want) {
			t.Fatalf("series %q not drawn: %q", want, out)
		}
	}
	// Each series drew exactly the one sample that carried it.
	for _, want := range []string{" 10%", " 90%", " 20%", " 80%"} {
		if !strings.Contains(out, want) {
			t.Errorf("series missing its sample's value %s: %q", want, out)
		}
	}
}

// The gauge format draws the current reading only, whatever the history
// holds — and a stopped engine's reading carries nothing, so it draws none.
func TestRenderStatGaugesIgnoresHistory(t *testing.T) {
	cpu := &metrics.CpuStat{Utilization: 50}
	var b bytes.Buffer
	renderStatGauges(&b, cpu, nil, nil, nil, nil)
	if !strings.Contains(b.String(), " 50%") || strings.Contains(b.String(), "▁") {
		t.Errorf("gauge drew history or the wrong value: %q", b.String())
	}
	b.Reset()
	renderStatGauges(&b, nil, nil, nil, nil, nil)
	if b.String() != "" {
		t.Errorf("a stopped engine drew gauges: %q", b.String())
	}
}

// The serve view's format draws both halves together: each series as a gauge
// of its current reading with its retained history as a sparkline beneath,
// the gauge carrying the label and the sparkline a blank one, so the pair
// stacks in the label column.
func TestRenderStatCombined(t *testing.T) {
	cpu := &metrics.CpuStat{Utilization: 42}
	mem := &metrics.MemoryStat{Total: 1000, Used: 300}
	gpus := []metrics.GpuStat{{Index: 0, Name: "H100", Utilization: 61, MemoryUsed: 80, MemoryTotal: 160}}
	history := []metrics.HistorySample{
		{Time: 1, CPU: ptrPct(10), Mem: ptrPct(20), GPUs: []metrics.HistoryGPU{{Index: 0, Util: 30, Mem: ptrPct(10)}}},
		{Time: 2, CPU: ptrPct(95), Mem: ptrPct(30), GPUs: []metrics.HistoryGPU{{Index: 0, Util: 91, Mem: ptrPct(20)}}},
	}
	var b bytes.Buffer
	renderStatCombined(&b, cpu, mem, gpus, nil, history)
	lines := strings.Split(strings.TrimSuffix(b.String(), "\n"), "\n")
	// One line per series, in the bar series' order.
	if len(lines) != 4 {
		t.Fatalf("drew %d lines, want 4: %q", len(lines), b.String())
	}
	for i, label := range []string{"  CPU     ", "  RAM     ", "  GPU     ", "  GPU mem "} {
		if !strings.HasPrefix(lines[i], label) {
			t.Errorf("line for %s: %q", label, lines[i])
		}
	}
	// The line is label, gauge half, figure, bar half — the figure between
	// the two halves, and it is the current reading, not the bar's last
	// sample.
	cpuBar, _ := sparklineBlock([]float64{10, 95}, serveBarW)
	wantCPU := "  CPU     " + gaugeBlock(42, serveGaugeW) + "  42% " + cpuBar
	if lines[0] != wantCPU {
		t.Errorf("CPU line:\ngot  %q\nwant %q", lines[0], wantCPU)
	}
	// Both halves take the state colour: the gauge over its whole fill, the
	// sparkline on the point that crosses the threshold — and the sparkline
	// stops one grade short of the full block, so the 95% sample draws the
	// top of the seven sub-full glyphs. Two colour pairs only: one around
	// the gauge's fill, one around the red spike.
	if !strings.Contains(lines[0], ansiGreen+strings.Repeat("█", 8)+ansiReset+strings.Repeat("░", 12)) {
		t.Errorf("CPU gauge: %q", lines[0])
	}
	if !strings.Contains(lines[0], ansiRed+"▇"+ansiReset) {
		t.Errorf("CPU sparkline: %q", lines[0])
	}
	if n := strings.Count(lines[0], "\033["); n != 4 {
		t.Errorf("CPU line has %d escape sequences, want 4: %q", n, lines[0])
	}
	// Every line is label, gauge half, figure and bar half: the halves sit
	// side by side, aligned across the series. The width is label (12),
	// gauge half, a space, the figure's fixed-width field (4: three digits
	// right-aligned plus the percent sign), a space, bar half.
	for i, line := range lines {
		if w := lipgloss.Width(line); w != 10+serveGaugeW+1+4+1+serveBarW {
			t.Errorf("line %d is %d columns wide, want %d: %q", i, w, 10+serveGaugeW+1+4+1+serveBarW, line)
		}
	}
}

// The figure is a fixed-width field, so the bar half starts in the same
// column whatever the reading's digit count: a value crossing a digit
// boundary keeps the line the same width instead of shifting the series'
// bar.
func TestRenderStatCombinedFigureDoesNotShiftTheBar(t *testing.T) {
	history := []metrics.HistorySample{
		{Time: 1, CPU: ptrPct(10)},
		{Time: 2, CPU: ptrPct(40)},
	}
	cpuBar, _ := sparklineBlock([]float64{10, 40}, serveBarW)
	cases := []struct {
		value  float64
		figure string
	}{
		{0, "  0%"}, {7, "  7%"}, {42, " 42%"}, {100, "100%"},
	}
	for _, c := range cases {
		var b bytes.Buffer
		renderStatCombined(&b, &metrics.CpuStat{Utilization: c.value}, nil, nil, nil, history)
		lines := strings.Split(strings.TrimSuffix(b.String(), "\n"), "\n")
		if len(lines) != 1 {
			t.Fatalf("figure %s drew %d lines, want 1: %q", c.figure, len(lines), b.String())
		}
		want := "  CPU     " + gaugeBlock(c.value, serveGaugeW) + " " + c.figure + " " + cpuBar
		if lines[0] != want {
			t.Errorf("figure %s:\ngot  %q\nwant %q", c.figure, lines[0], want)
		}
		if w := lipgloss.Width(lines[0]); w != 10+serveGaugeW+1+4+1+serveBarW {
			t.Errorf("figure %s drew a %d-column line, want %d: %q", c.figure, w, 10+serveGaugeW+1+4+1+serveBarW, lines[0])
		}
	}
}

// A memory reading with no total reports 0, not a division by it, in either
// half of the combined format.
func TestRenderStatCombinedMemoryWithoutTotal(t *testing.T) {
	var b bytes.Buffer
	renderStatCombined(&b, nil, &metrics.MemoryStat{Total: 0, Used: 100}, nil, nil, nil)
	if !strings.Contains(b.String(), "  0%") || strings.Contains(b.String(), "NaN") {
		t.Errorf("a memory reading with no total: %q", b.String())
	}
}

// Without retained history each series carries its gauge alone, its bar
// half left blank.
func TestRenderStatCombinedWithoutHistory(t *testing.T) {
	cpu := &metrics.CpuStat{Utilization: 42}
	mem := &metrics.MemoryStat{Total: 1000, Used: 300}
	gpus := []metrics.GpuStat{{Index: 0, Name: "H100", Utilization: 61, MemoryUsed: 80, MemoryTotal: 160}}
	var b bytes.Buffer
	renderStatCombined(&b, cpu, mem, gpus, nil, nil)
	lines := strings.Split(strings.TrimSuffix(b.String(), "\n"), "\n")
	blankBar := strings.Repeat(" ", serveBarW)
	want := []string{
		"  CPU     " + gaugeBlock(42, serveGaugeW) + "  42% " + blankBar,
		"  RAM     " + gaugeBlock(30, serveGaugeW) + "  30% " + blankBar,
		"  GPU     " + gaugeBlock(61, serveGaugeW) + "  61% " + blankBar,
		"  GPU mem " + gaugeBlock(50, serveGaugeW) + "  50% " + blankBar,
	}
	if len(lines) != len(want) {
		t.Fatalf("drew %d lines, want %d: %q", len(lines), len(want), b.String())
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d:\ngot  %q\nwant %q", i, lines[i], want[i])
		}
	}
}

// A stopped engine carries no current figures, so the combined format draws
// the retained readings alone — the sparkline lines with their blank label
// column, no gauges at all.
// A stopped engine carries no current figures, so the combined format draws
// each series' history alone: the gauge half blank, the series' label kept,
// and the bar's latest sample as the line's figure.
func TestRenderStatCombinedStoppedEngineDrawsHistoryAlone(t *testing.T) {
	history := []metrics.HistorySample{
		{Time: 1, CPU: ptrPct(10), Mem: ptrPct(20), GPUs: []metrics.HistoryGPU{{Index: 0, Util: 50, Mem: ptrPct(50)}}},
		{Time: 2, CPU: ptrPct(20), Mem: ptrPct(30), GPUs: []metrics.HistoryGPU{{Index: 0, Util: 60, Mem: ptrPct(60)}}},
	}
	var b bytes.Buffer
	renderStatCombined(&b, nil, nil, nil, nil, history)
	lines := strings.Split(strings.TrimSuffix(b.String(), "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("drew %d lines, want CPU, RAM, GPU and GPU mem: %q", len(lines), b.String())
	}
	blankGauge := strings.Repeat(" ", serveGaugeW)
	wantPrefix := []string{
		"  CPU     " + blankGauge + "  20% ",
		"  RAM     " + blankGauge + "  30% ",
		"  GPU     " + blankGauge + "  60% ",
		"  GPU mem " + blankGauge + "  60% ",
	}
	for i, want := range wantPrefix {
		if !strings.HasPrefix(lines[i], want) {
			t.Errorf("line %d:\ngot  %q\nwant prefix %q", i, lines[i], want)
		}
	}
	// The bar ends the line, its two samples as its two right-most glyphs;
	// every reading sits below the thresholds, so none takes a colour.
	wantSuffix := []string{"▁▂", "▂▃", "▄▅", "▄▅"}
	for i, want := range wantSuffix {
		if !strings.HasSuffix(lines[i], want) {
			t.Errorf("line %d must end with the bar's two samples:\ngot %q", i, lines[i])
		}
	}
	if strings.Contains(b.String(), "\033[") {
		t.Errorf("a below-threshold point took a colour:\n%s", b.String())
	}
}

func TestFormatMetricsBarStoppedWithHistory(t *testing.T) {
	results := []fleet.NodeResult{{
		Name: "prod", Outcome: fleet.OutcomeOK,
		Metrics: metrics.Stats{
			State: "stopped", ModelID: "org/qwen:q4",
			LastActiveAt: "2026-08-21T10:00:00Z", IdleSeconds: 12,
			History: []metrics.HistorySample{
				{Time: 1, CPU: ptrPct(10)},
				{Time: 2, CPU: ptrPct(20)},
			},
		},
	}}
	var b bytes.Buffer
	if err := renderFleetMetrics(&b, results, "bar"); err != nil {
		t.Fatal(err)
	}
	want := "prod  stopped  org/qwen:q4\n" +
		"  active  12s ago\n" +
		"  CPU     " + strings.Repeat(" ", 38) + "▁▂" + " 20%\n"
	if got := b.String(); got != want {
		t.Errorf("stopped bar = %q, want %q", got, want)
	}

	// The gauge format draws no series for a stopped endpoint: the header and
	// the active line, and nothing after.
	b.Reset()
	if err := renderFleetMetrics(&b, results, "gauge"); err != nil {
		t.Fatal(err)
	}
	want = "prod  stopped  org/qwen:q4\n" +
		"  active  12s ago\n"
	if got := b.String(); got != want {
		t.Errorf("stopped gauge = %q, want %q", got, want)
	}
}

func TestFormatMetricsBarRunning(t *testing.T) {
	resp := &cloud.StatsResponse{
		Environment: "prod", State: "running", InstanceType: "g5.xlarge",
		ModelID: "org/qwen:q4", Version: "0.4.3",
		LastActiveAt: "2026-08-21T10:00:00Z", IdleSeconds: 3,
		CPU:    &metrics.CpuStat{Utilization: 62},
		Memory: &metrics.MemoryStat{Total: 1000, Used: 300},
		GPUs:   []metrics.GpuStat{{Index: 0, Name: "H100", Utilization: 61, MemoryUsed: 80, MemoryTotal: 160}},
		Tokens: &cloud.TokenStats{Running: 2, PromptTokens: 4096, GenerationTokens: 1024, Requests: ptrInt(17)},
		History: []metrics.HistorySample{
			{Time: 1, CPU: ptrPct(10), Mem: ptrPct(20), GPUs: []metrics.HistoryGPU{{Index: 0, Util: 50, Mem: ptrPct(50)}}},
			{Time: 2, CPU: ptrPct(20), Mem: ptrPct(30), GPUs: []metrics.HistoryGPU{{Index: 0, Util: 61, Mem: ptrPct(50)}}},
		},
	}
	var b bytes.Buffer
	if err := renderFleetMetrics(&b, nodeResultsFor(resp), "bar"); err != nil {
		t.Fatal(err)
	}
	got := b.String()
	if !strings.HasPrefix(got, "prod  running  g5.xlarge  org/qwen:q4  0.4.3\n") {
		t.Errorf("header: %q", got)
	}
	if !strings.Contains(got, "  active  3s ago\n") {
		t.Errorf("active line missing: %q", got)
	}
	// Every series drew a sparkline from the history: no gauge in the output.
	if strings.Contains(got, "░") {
		t.Errorf("a series fell back to the gauge although history holds it: %q", got)
	}
	for _, label := range []string{"CPU", "RAM", "GPU", "GPU mem"} {
		if !strings.Contains(got, label) {
			t.Errorf("series %q missing: %q", label, got)
		}
	}
	if !strings.Contains(got, "  running:          2\n") || !strings.Contains(got, "  requests:         17\n") {
		t.Errorf("token block missing: %q", got)
	}
}

// An engine family whose metrics expose no cumulative request counter yields
// statistics without the figure, and the token block draws no line for it.
func TestFormatMetricsBarNoRequestCount(t *testing.T) {
	resp := &cloud.StatsResponse{
		Environment: "prod", State: "running", InstanceType: "g5.xlarge",
		ModelID: "org/qwen:q4", Version: "0.4.3",
		Tokens: &cloud.TokenStats{Running: 2, PromptTokens: 4096, GenerationTokens: 1024},
	}
	var b bytes.Buffer
	if err := renderFleetMetrics(&b, nodeResultsFor(resp), "bar"); err != nil {
		t.Fatal(err)
	}
	got := b.String()
	if !strings.Contains(got, "  running:          2\n") {
		t.Errorf("running line missing: %q", got)
	}
	if strings.Contains(got, "  requests:") {
		t.Errorf("requests line drawn for an engine that exposes no request count: %q", got)
	}
}

func TestFormatMetricsJSONCarriesHistory(t *testing.T) {
	resp := &cloud.StatsResponse{
		Environment: "prod", State: "running",
		CPU: &metrics.CpuStat{Utilization: 62},
		History: []metrics.HistorySample{
			{Time: 1786276800, CPU: ptrPct(62), GPUs: []metrics.HistoryGPU{{Index: 0, Util: 61, Mem: ptrPct(50)}}},
		},
	}
	var b bytes.Buffer
	if err := renderFleetMetrics(&b, nodeResultsFor(resp), "json"); err != nil {
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
	if err := renderFleetMetrics(&b, nodeResultsFor(resp), "json"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(b.String(), "history") {
		t.Errorf("absent history serialised: %s", b.String())
	}
}

// nodeResultsFor turns a control-plane stats reply into the one node result a
// fan-out would produce for it, so a test can state its input as the reply and
// assert on what the renderer draws.
func nodeResultsFor(resp *cloud.StatsResponse) []fleet.NodeResult {
	return []fleet.NodeResult{{
		Name: resp.Environment, Outcome: fleet.OutcomeOK,
		Instance: fleet.Instance{
			ID: resp.InstanceID, Type: resp.InstanceType, Version: resp.Version,
			RetainUntil: resp.RetainUntil,
		},
		Metrics: metrics.Stats{
			State: resp.State, Runner: resp.Runner, ModelID: resp.ModelID,
			UptimeSeconds: resp.UptimeSeconds, Tokens: resp.Tokens,
			GPUs: resp.GPUs, CPU: resp.CPU, Memory: resp.Memory,
			History: resp.History, Errors: resp.Errors,
			LastActiveAt: resp.LastActiveAt, IdleSeconds: resp.IdleSeconds,
			RetainUntil: resp.RetainUntil,
		},
	}}
}

// The Speed row: the sparkline starts in the bars' column and the t/s
// figure trails where the percentages sit, the window's high watermark sets
// the glyph heights, every point draws in the default colour, and a figure
// that has lapsed reads zero while the window still remembers the rate.
func TestRenderSpeedRow(t *testing.T) {
	speed := 8.5
	history := []metrics.HistorySample{
		{Time: 1, Speed: ptrPct(2.0)},
		{Time: 2, Speed: ptrPct(5.0)},
		{Time: 3, Speed: ptrPct(10.0)},
	}
	bar := strings.Repeat(" ", barLineW-3) + "▂▄▇"
	var b bytes.Buffer
	renderStatBars(&b, &metrics.CpuStat{Utilization: 95}, nil, nil, &speed, history, barLineW)
	lines := strings.Split(strings.TrimSuffix(b.String(), "\n"), "\n")
	line := lines[len(lines)-1]
	want := "  Speed   " + bar + " 8.50 t/s"
	if line != want {
		t.Fatalf("speed row:\ngot  %q\nwant %q", line, want)
	}
	// The sparkline shares the CPU row's graph column: same label column,
	// same draw width, so the glyphs start at the same character.
	if got, cpu := line[10:20], lines[0][10:20]; len(got) != len(cpu) {
		t.Errorf("graph columns differ: %q / %q", got, cpu)
	}
	// The window's fastest point takes the heaviest grade the sparkline
	// draws, its slowest one far lighter — and neither the full block.
	if !strings.HasSuffix(bar, "▇") || !strings.Contains(bar, "▂") {
		t.Errorf("watermark mapping: %q", bar)
	}
	// The speed row draws no threshold colour at its window maximum — while
	// the 95% CPU row on the same screen is free to be coloured.
	if strings.Contains(line, "\x1b[") {
		t.Errorf("the speed row drew colour: %q", line)
	}
	if !strings.Contains(lines[0], "\x1b[") {
		t.Errorf("the CPU row should still colour: %q", lines[0])
	}

	// A stopped engine: its history froze at the stop, so the retained
	// burst still sits at the right edge and the figure reads zero — the
	// row tells what the engine was doing until it stopped.
	b.Reset()
	renderStatBars(&b, nil, nil, nil, nil, history, barLineW)
	line = strings.Split(strings.TrimSuffix(b.String(), "\n"), "\n")[0]
	if want := "  Speed   " + bar + " 0.00 t/s"; line != want {
		t.Errorf("a stopped engine's speed row:\ngot  %q\nwant %q", line, want)
	}

	// The lapse in the making: the engine still runs, so quiet ticks keep
	// landing in the window as speedless samples. Each one slides the
	// burst a column left and leaves its own blank at the right edge, so
	// the sparkline's now says the same thing as the figure's zero.
	b.Reset()
	quiet := append(append([]metrics.HistorySample{}, history...),
		metrics.HistorySample{Time: 4}, metrics.HistorySample{Time: 5})
	renderStatBars(&b, nil, nil, nil, nil, quiet, barLineW)
	line = strings.Split(strings.TrimSuffix(b.String(), "\n"), "\n")[0]
	if want := "  Speed   " + strings.Repeat(" ", barLineW-5) + "▂▄▇  " + " 0.00 t/s"; line != want {
		t.Errorf("an engine gone quiet's speed row:\ngot  %q\nwant %q", line, want)
	}

	// Neither reading nor history: no row.
	b.Reset()
	renderStatBars(&b, &metrics.CpuStat{Utilization: 10}, nil, nil, nil, nil, barLineW)
	if strings.Contains(b.String(), "Speed") {
		t.Errorf("an engine without speed drew a row: %q", b.String())
	}
}

// The gauge format and the serve view carry the figure where the graphs
// start — the rate has no ceiling to fill a gauge against — and the serve
// view keeps its sparkline in the sparklines' column however many digits the
// figure grows.
func TestRenderSpeedFigureOnly(t *testing.T) {
	speed := 12.34
	var b bytes.Buffer
	renderStatGauges(&b, nil, nil, nil, &speed, nil)
	if got := b.String(); got != "  Speed   12.34 t/s\n" {
		t.Errorf("gauge speed row: %q", got)
	}

	// Lapsed but reporting, the gauge-format row says 0.00 rather than
	// vanishing under a watching eye.
	b.Reset()
	renderStatGauges(&b, nil, nil, nil, nil, []metrics.HistorySample{{Time: 1, Speed: ptrPct(4.0)}})
	if got := b.String(); got != "  Speed   0.00 t/s\n" {
		t.Errorf("lapsed gauge speed row: %q", got)
	}

	// The combined row: figure at the graph column, sparkline at the
	// sparkline column, pinned there across digit counts.
	for _, c := range []struct {
		value float64
		want  string
	}{
		{12.34, "12.34 t/s"},
		{5.00, "5.00 t/s"},
		{1234.56, "1234.56 t/s"},
	} {
		v := c.value
		b.Reset()
		renderStatCombined(&b, nil, nil, nil, &v, nil)
		line := strings.TrimSuffix(b.String(), "\n")
		want := "  Speed   " + fmt.Sprintf("%-26s", c.want) + strings.Repeat(" ", serveBarW)
		if line != want {
			t.Errorf("combined speed row %v:\ngot  %q\nwant %q", c.value, line, want)
		}
	}

	// No speed anywhere: neither surface mentions it.
	b.Reset()
	renderStatGauges(&b, nil, nil, nil, nil, nil)
	if strings.Contains(b.String(), "Speed") {
		t.Errorf("no figure still drew a gauge row: %q", b.String())
	}
	b.Reset()
	renderStatCombined(&b, nil, nil, nil, nil, nil)
	if strings.Contains(b.String(), "Speed") {
		t.Errorf("no figure still drew a combined row: %q", b.String())
	}
}
