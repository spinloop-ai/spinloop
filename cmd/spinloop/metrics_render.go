// Shared metrics rendering. Both `spinloop remote metrics` (one cloud endpoint)
// and `spinloop fleet metrics` (a node per machine) display the same
// internal/metrics stats, so the parts that draw those stats live here and
// each caller supplies only its own heading — the environment and instance
// type for remote, the node name for fleet.

package main

import (
	"fmt"
	"io"

	"github.com/spinloop-ai/spinloop/internal/metrics"
)

// lastActiveText is the shared phrase for how long ago an engine last did
// work — "12s ago" — or "" when there is nothing to report.
//
// The gate is the timestamp, never the seconds. idleSeconds is omitted at
// zero, so an engine working this instant carries a lastActiveAt and no
// duration; gating on the number would hide the busiest engine there is.
//
// The wording deliberately avoids "idle": that word is already an engine
// state meaning nothing has been started, and one screen should not carry two
// meanings of it. It matches `spinloop fleet status`, which has shown this fact
// since the daemon began tracking it.
func lastActiveText(lastActiveAt string, idleSeconds int) string {
	if lastActiveAt == "" {
		return ""
	}
	return formatDuration(idleSeconds) + " ago"
}

// renderLastActiveIndented draws the last-active line in the indented block
// the bar format and both fleet formats use, aligned to the bar-label column.
//
// Not a bar itself: an elapsed time has no ceiling to fill against, and a bar
// would imply one.
func renderLastActiveIndented(w io.Writer, lastActiveAt string, idleSeconds int) {
	if text := lastActiveText(lastActiveAt, idleSeconds); text != "" {
		fmt.Fprintf(w, "  %-9s %s\n", "last active", text)
	}
}

// renderLastActiveKeyValue draws the same fact as a row of the table format,
// padded to the key column its neighbours use.
func renderLastActiveKeyValue(w io.Writer, lastActiveAt string, idleSeconds int) {
	if text := lastActiveText(lastActiveAt, idleSeconds); text != "" {
		fmt.Fprintf(w, "last active:  %s\n", text)
	}
}

// renderRetainIndented draws the retention-deadline line in the indented block
// the bar format and both fleet formats use, aligned to the bar-label column
// and beside the last-active line. The deadline is an absolute instant the
// control plane set, relayed verbatim — it is not re-checked here, so a line is
// drawn exactly when the read carries one (the stats reply drops it once it has
// passed). Local daemon nodes carry no deadline, so their line is absent.
func renderRetainIndented(w io.Writer, retainUntil string) {
	if retainUntil != "" {
		fmt.Fprintf(w, "  %-9s %s\n", "retain until", retainUntil)
	}
}

// renderRetainKeyValue draws the retention deadline as a row of the table
// format, padded to the key column its neighbours use, beside the last-active
// row.
func renderRetainKeyValue(w io.Writer, retainUntil string) {
	if retainUntil != "" {
		fmt.Fprintf(w, "retain until: %s\n", retainUntil)
	}
}

// validateMetricsFormat rejects a --format value the metrics commands do not
// understand, naming the ones they do. Both `remote metrics` and
// `fleet metrics` run it before doing any work.
func validateMetricsFormat(format string) error {
	switch format {
	case "bar", "gauge", "table", "json":
		return nil
	}
	return fmt.Errorf("--format must be \"bar\", \"gauge\", \"table\", or \"json\", got %q", format)
}

// barLineW is the sparkline's draw width in the full view: the width of the
// window at the default sampler cadence, so a steady engine fills it and a
// fresh engine's still-filling window reads as leading space.
const barLineW = 40

// dashBarLineW is the sparkline's draw width inside a dashboard tile: the
// tile's width minus the label column and the widest trailing figure, so a
// full row fits the tile exactly and the clip never takes the percentage.
const dashBarLineW = 25

// barGlyphs is the eight block elements the sparkline draws with, lightest to
// heaviest: a series' value maps to the one whose fill height is nearest. A
// rune slice, not a string — the elements are multibyte, so byte indexing
// would not land on glyph boundaries.
var barGlyphs = []rune("▁▂▃▄▅▆▇█")

// barGlyph picks the block element for a 0-100% value.
func barGlyph(pct float64) rune {
	i := int(pct / 100.0 * float64(len(barGlyphs)))
	if i > len(barGlyphs)-1 {
		i = len(barGlyphs) - 1
	}
	return barGlyphs[i]
}

// poolMax reduces a series to at most width values, one per column, each the
// maximum of the samples pooled into its column. A sparkline is about
// pressure and the thresholds are about peaks, so a pool keeps its highest
// reading rather than averaging one away — an average would flatten the red
// spike the series exists to show.
func poolMax(values []float64, width int) []float64 {
	if len(values) <= width {
		return values
	}
	out := make([]float64, width)
	per := float64(len(values)) / float64(width)
	for c := range out {
		lo := int(float64(c) * per)
		hi := int(float64(c+1) * per)
		if c == width-1 {
			// The split's rounding can leave the newest sample outside the
			// final column, and the trailing figure is that sample's value —
			// so the last column takes it whatever the arithmetic says.
			hi = len(values)
		}
		m := values[lo]
		for i := lo + 1; i < hi; i++ {
			if values[i] > m {
				m = values[i]
			}
		}
		out[c] = m
	}
	return out
}

// renderSparkline draws one resource series across width columns, one glyph
// per sample, newest on the right. The window's leading columns are blank
// while it still fills, the final glyph takes the state colour on the
// gauge's 80/90 thresholds, and the trailing figure is the latest sample's
// percentage — the exact value the last glyph approximates.
func renderSparkline(w io.Writer, label string, samples []float64, width int) {
	pooled := poolMax(samples, width)
	last := pooled[len(pooled)-1]
	colour := ansiGreen
	if last > 90 {
		colour = ansiRed
	} else if last >= 80 {
		colour = ansiYellow
	}
	fmt.Fprintf(w, "  %-9s ", label)
	for i := 0; i < width-len(pooled); i++ {
		fmt.Fprint(w, " ")
	}
	for i, v := range pooled {
		if i == len(pooled)-1 {
			fmt.Fprintf(w, "%s%c%s", colour, barGlyph(v), ansiReset)
		} else {
			fmt.Fprintf(w, "%c", barGlyph(v))
		}
	}
	fmt.Fprintf(w, " %.0f%%\n", last)
}

// renderGauge draws one resource series as a horizontal progress gauge: the
// filled portion in the state colour, the unfilled portion in light shade,
// the percentage in the terminal's default colour. It draws the current
// reading only — it carries no history.
func renderGauge(w io.Writer, label string, pct float64) {
	const width = 25
	colour := ansiGreen
	if pct > 90 {
		colour = ansiRed
	} else if pct >= 80 {
		colour = ansiYellow
	}
	filled := int(pct / 100.0 * float64(width))
	if filled > width {
		filled = width
	}
	empty := width - filled
	fmt.Fprintf(w, "  %-9s ", label)
	fmt.Fprintf(w, "%s", colour)
	for i := 0; i < filled; i++ {
		fmt.Fprint(w, "█")
	}
	fmt.Fprintf(w, "%s", ansiReset)
	for i := 0; i < empty; i++ {
		fmt.Fprint(w, "░")
	}
	fmt.Fprintf(w, " %.0f%%\n", pct)
}

// barSeries is one resource series the bar and gauge formats draw: the label
// the gauge uses for it, the retained percentage readings oldest first, and
// the current reading the gauge fallback draws where the history holds none.
type barSeries struct {
	label   string
	history []float64
	current *float64
}

// barSeriesList works out which series the reading and the history carry, in
// the order the gauge draws them: CPU, RAM, then each GPU's utilisation and
// memory. The GPU set is the union of the two sources in the current
// reading's order first — a stopped engine's current reading names no GPU,
// but its retained readings still name the ones it ran on, and the series
// those readings carry are the point of keeping them.
func barSeriesList(cpu *metrics.CpuStat, mem *metrics.MemoryStat, gpus []metrics.GpuStat, history []metrics.HistorySample) []barSeries {
	var out []barSeries
	add := func(label string, current *float64, pick func(s metrics.HistorySample) *float64) {
		var vals []float64
		for _, s := range history {
			if v := pick(s); v != nil {
				vals = append(vals, *v)
			}
		}
		if current == nil && len(vals) == 0 {
			return
		}
		out = append(out, barSeries{label: label, history: vals, current: current})
	}
	if cpu != nil {
		v := cpu.Utilization
		add("CPU", &v, func(s metrics.HistorySample) *float64 { return s.CPU })
	} else {
		add("CPU", nil, func(s metrics.HistorySample) *float64 { return s.CPU })
	}
	if mem != nil {
		pct := 0.0
		if mem.Total > 0 {
			pct = float64(mem.Used) / float64(mem.Total) * 100
		}
		add("RAM", &pct, func(s metrics.HistorySample) *float64 { return s.Mem })
	} else {
		add("RAM", nil, func(s metrics.HistorySample) *float64 { return s.Mem })
	}
	seen := map[int]bool{}
	var idxs []int
	for _, g := range gpus {
		if !seen[g.Index] {
			seen[g.Index] = true
			idxs = append(idxs, g.Index)
		}
	}
	for _, s := range history {
		for _, g := range s.GPUs {
			if !seen[g.Index] {
				seen[g.Index] = true
				idxs = append(idxs, g.Index)
			}
		}
	}
	for _, idx := range idxs {
		prefix := "GPU"
		if len(idxs) > 1 {
			prefix = fmt.Sprintf("GPU %d", idx)
		}
		add(prefix+" util", currentGPUUtil(gpus, idx), func(s metrics.HistorySample) *float64 {
			for _, g := range s.GPUs {
				if g.Index == idx {
					v := float64(g.Util)
					return &v
				}
			}
			return nil
		})
		add(prefix+" mem", currentGPUMem(gpus, idx), func(s metrics.HistorySample) *float64 {
			for _, g := range s.GPUs {
				if g.Index == idx {
					return g.Mem
				}
			}
			return nil
		})
	}
	return out
}

// currentGPUUtil is the GPU's utilisation from the current reading, nil where
// the reading names no such GPU — the stopped engine's case.
func currentGPUUtil(gpus []metrics.GpuStat, idx int) *float64 {
	for _, g := range gpus {
		if g.Index == idx {
			v := float64(g.Utilization)
			return &v
		}
	}
	return nil
}

// currentGPUMem is the GPU's memory ratio from the current reading, 0 where
// the GPU reports no total — the gauge's own rule for that case.
func currentGPUMem(gpus []metrics.GpuStat, idx int) *float64 {
	for _, g := range gpus {
		if g.Index == idx {
			pct := 0.0
			if g.MemoryTotal > 0 {
				pct = float64(g.MemoryUsed) / float64(g.MemoryTotal) * 100
			}
			return &pct
		}
	}
	return nil
}

// renderStatBars draws the resource series in the bar format: each series as
// a sparkline of the retained history, or — where the history holds none for
// the series — as the gauge drawing of the current reading, so a daemon that
// predates the history renders exactly as it did before it. Used by the bar
// format on every surface; lineW is the draw width, barLineW for the full
// view and dashBarLineW for a dashboard tile.
func renderStatBars(w io.Writer, cpu *metrics.CpuStat, mem *metrics.MemoryStat, gpus []metrics.GpuStat, history []metrics.HistorySample, lineW int) {
	for _, s := range barSeriesList(cpu, mem, gpus, history) {
		if len(s.history) > 0 {
			renderSparkline(w, s.label, s.history, lineW)
		} else {
			renderGauge(w, s.label, *s.current)
		}
	}
}

// renderStatGauges draws the resource series in the gauge format: the current
// reading only, whatever the history holds.
func renderStatGauges(w io.Writer, cpu *metrics.CpuStat, mem *metrics.MemoryStat, gpus []metrics.GpuStat) {
	for _, s := range barSeriesList(cpu, mem, gpus, nil) {
		if s.current != nil {
			renderGauge(w, s.label, *s.current)
		}
	}
}

// renderTokenLines draws the engine's token and request counters, the block
// both formats share.
func renderTokenLines(w io.Writer, tokens *metrics.TokenStats) {
	if tokens == nil {
		return
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  running:          %d\n", tokens.Running)
	fmt.Fprintf(w, "  prompt tokens:    %d\n", tokens.PromptTokens)
	fmt.Fprintf(w, "  generation tokens: %d\n", tokens.GenerationTokens)
	fmt.Fprintf(w, "  requests:         %d\n", tokens.Requests)
}

// renderGPUTable draws the per-GPU lines of the table format, plus the
// totals line when there is more than one GPU.
func renderGPUTable(w io.Writer, gpus []metrics.GpuStat) {
	if len(gpus) == 0 {
		return
	}
	fmt.Fprintln(w)
	for _, g := range gpus {
		fmt.Fprintf(w, "  GPU %d: %s  util=%d%%  mem=%s/%s  temp=%dC\n",
			g.Index, g.Name, g.Utilization, formatBytes(g.MemoryUsed), formatBytes(g.MemoryTotal), g.Temperature)
	}
	if len(gpus) > 1 {
		var totalUtil, totalMemUsed, totalMemTotal int64
		for _, g := range gpus {
			totalUtil += int64(g.Utilization)
			totalMemUsed += g.MemoryUsed
			totalMemTotal += g.MemoryTotal
		}
		avgUtil := int(totalUtil) / len(gpus)
		fmt.Fprintf(w, "  avg util: %d%%  total mem: %s/%s\n",
			avgUtil, formatBytes(totalMemUsed), formatBytes(totalMemTotal))
	}
}

// renderCPUMemTable draws the table format's CPU and RAM lines.
func renderCPUMemTable(w io.Writer, cpu *metrics.CpuStat, mem *metrics.MemoryStat) {
	if cpu != nil {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  CPU: %.0f%% util\n", cpu.Utilization)
	}
	if mem != nil {
		pct := float64(mem.Used) / float64(mem.Total) * 100
		fmt.Fprintf(w, "  RAM: %s/%s (%.0f%%)\n", formatBytes(mem.Used), formatBytes(mem.Total), pct)
	}
}

// renderCollectionErrors reports metric-collection problems on stderr, so the
// data on stdout stays clean for piping.
func renderCollectionErrors(w io.Writer, errs []string) {
	if len(errs) == 0 {
		return
	}
	fmt.Fprintln(w, "metric collection errors:")
	for _, e := range errs {
		fmt.Fprintf(w, "  - %s\n", e)
	}
}
