package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	teatest "github.com/charmbracelet/x/exp/teatest"
	"github.com/muesli/termenv"
	"github.com/spinloop-ai/spinloop/internal/daemon"
	"github.com/spinloop-ai/spinloop/internal/fleet"
	"github.com/spinloop-ai/spinloop/internal/metrics"
	"github.com/spinloop-ai/spinloop/internal/remote"
)

// fakeDashNode is one in-memory fleet node: Start and Stop flip its state and
// its Metrics answer follows, so a program-level test can watch a tile turn
// on and off. metricsErr makes the node answer no metrics at all. A non-nil
// hold keeps StartWithProgress on its first status line until the test
// closes it — an in-flight start the test can watch.
type fakeDashNode struct {
	mu         sync.Mutex
	state      string
	metricsErr error
	starts     int
	stops      int
	hold       <-chan struct{}
	startErr   error                  // Start refuses with this
	startReply *daemon.StatusResponse // Start answers with this instead of running
	log        []byte                 // the engine log Logs reads slices of
	logErr     error                  // Logs answers this instead
}

func newFakeDashNode(state string) *fakeDashNode { return &fakeDashNode{state: state} }

func (f *fakeDashNode) failMetrics(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.metricsErr = err
}

// appendLog simulates the engine writing more output, for a test to watch a
// follow pick up.
func (f *fakeDashNode) appendLog(s string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.log = append(f.log, []byte(s)...)
}

func (f *fakeDashNode) failLogs(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.logErr = err
}

func (f *fakeDashNode) Name() string { return "fake" }

func (f *fakeDashNode) Status(ctx context.Context) (daemon.StatusResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return daemon.StatusResponse{State: f.state}, nil
}

func (f *fakeDashNode) Metrics(ctx context.Context) (metrics.Stats, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.metricsErr != nil {
		return metrics.Stats{}, f.metricsErr
	}
	s := metrics.Stats{State: f.state, Runner: "llamacpp", ModelID: "org/qwen"}
	if f.state == "running" {
		s.UptimeSeconds = 60
		s.CPU = &metrics.CpuStat{Utilization: 42}
		s.GPUs = []metrics.GpuStat{{Index: 0, Name: "H100", Utilization: 10, MemoryUsed: 1, MemoryTotal: 10}}
		s.Tokens = &metrics.TokenStats{Running: 1, PromptTokens: 100, GenerationTokens: 50, Requests: 3}
	}
	return s, nil
}

func (f *fakeDashNode) Start(ctx context.Context) (daemon.StatusResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.starts++
	if f.startErr != nil {
		return daemon.StatusResponse{}, f.startErr
	}
	f.state = "running"
	if f.startReply != nil {
		return *f.startReply, nil
	}
	return daemon.StatusResponse{State: "running"}, nil
}

// plainDashNode wraps the fake without its progress capability: a node whose
// start is one POST and one reply, so the dashboard's type assertion finds no
// ProgressStarter and falls back to the plain Start.
type plainDashNode struct{ f *fakeDashNode }

func (n plainDashNode) Name() string { return n.f.Name() }

func (n plainDashNode) Status(ctx context.Context) (daemon.StatusResponse, error) {
	return n.f.Status(ctx)
}

func (n plainDashNode) Metrics(ctx context.Context) (metrics.Stats, error) { return n.f.Metrics(ctx) }

func (n plainDashNode) Start(ctx context.Context) (daemon.StatusResponse, error) {
	return n.f.Start(ctx)
}

func (n plainDashNode) StartWith(ctx context.Context, dc *remote.DeployConfig, engineKey string) (daemon.StatusResponse, error) {
	return n.f.StartWith(ctx, dc, engineKey)
}

func (n plainDashNode) Stop(ctx context.Context) (daemon.StatusResponse, error) {
	return n.f.Stop(ctx)
}

func (n plainDashNode) Logs(ctx context.Context, offset int64, limit int) (daemon.LogsResponse, error) {
	return n.f.Logs(ctx, offset, limit)
}

// The fake also reports one progress line on the way, so the dashboard's
// progress path is exercised end to end, through the program's Send — and,
// when told to hold, stays on that line until released. A wait that ends on
// the context — the abort — comes back with the context's error, the way the
// control plane's own loop does.
func (f *fakeDashNode) StartWithProgress(ctx context.Context, progress func(string)) (daemon.StatusResponse, error) {
	progress("instance starting; retrying in 1s")
	if f.hold != nil {
		select {
		case <-f.hold:
		case <-ctx.Done():
			return daemon.StatusResponse{}, ctx.Err()
		}
	}
	return f.Start(ctx)
}

func (f *fakeDashNode) StartWith(ctx context.Context, dc *remote.DeployConfig, engineKey string) (daemon.StatusResponse, error) {
	return daemon.StatusResponse{}, errors.New("not driven by the dashboard")
}

func (f *fakeDashNode) Stop(ctx context.Context) (daemon.StatusResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stops++
	f.state = "stopped"
	return daemon.StatusResponse{State: "stopped"}, nil
}

// Logs answers a slice of the fake log, the same shape daemon.ReadLog would:
// daemon.TailLog reads the last limit bytes (the backlog a fresh tail
// wants), and a stated offset resumes from there for up to limit bytes (or
// to the end, unstated) — so a test can drive the detail view's tail the
// same way the real endpoint would.
func (f *fakeDashNode) Logs(ctx context.Context, offset int64, limit int) (daemon.LogsResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.logErr != nil {
		return daemon.LogsResponse{}, f.logErr
	}
	size := int64(len(f.log))
	if limit <= 0 {
		limit = 1 << 20
	}
	start := offset
	if start == daemon.TailLog {
		if start = size - int64(limit); start < 0 {
			start = 0
		}
	}
	if start < 0 {
		start = 0
	}
	if start > size {
		return daemon.LogsResponse{NextOffset: size, Size: size, StaleOffset: true}, nil
	}
	end := start + int64(limit)
	if end > size {
		end = size
	}
	return daemon.LogsResponse{Content: string(f.log[start:end]), NextOffset: end, Size: size}, nil
}

// startFastRound begins the local group's round through the model's own
// door and runs it to completion.
func startFastRound(t *testing.T, m *dashModel) (dashRefreshMsg, bool) {
	t.Helper()
	cmd := m.refreshRemoteGroup(false)
	if cmd == nil {
		return dashRefreshMsg{}, false
	}
	out := cmd()
	r, ok := out.(dashRefreshMsg)
	if !ok {
		t.Fatalf("fast round cmd answered %T, want dashRefreshMsg", out)
	}
	return r, true
}

// landRounds runs each round the model started to completion and applies its
// answer, one at a time.
func landRounds(t *testing.T, m *dashModel, cmds []tea.Cmd) *dashModel {
	t.Helper()
	for _, cmd := range cmds {
		out := cmd()
		msg, ok := out.(dashRefreshMsg)
		if !ok {
			t.Fatalf("round cmd answered %T, want dashRefreshMsg", out)
		}
		next, _ := m.Update(msg)
		m = next.(*dashModel)
	}
	return m
}

// dashKey is a KeyMsg for a one-keystroke string: the model and the test
// type the same words.
func dashKey(s string) tea.KeyMsg {
	switch s {
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

// The tile draw the bar-format helpers produce for a running node. The
// colours come from renderBar itself — they are content, not decoration — so
// the expected string carries them out, and the test pins the profile to
// Ascii so the frame alone is colour-stripped.

func dashBar(label string, pct float64) string {
	const width = 25
	colour := "\033[92m"
	if pct > 90 {
		colour = "\033[31m"
	} else if pct >= 80 {
		colour = "\033[33m"
	}
	filled := int(pct / 100 * width)
	return fmt.Sprintf("  %-9s ", label) + colour +
		strings.Repeat("█", filled) + "\033[0m" +
		strings.Repeat("░", width-filled) + fmt.Sprintf(" %.0f%%", pct)
}

func dashTileExpected(lines []string) string {
	var b strings.Builder
	b.WriteString("╭" + strings.Repeat("─", dashTileW) + "╮\n")
	for _, line := range lines {
		if pad := dashTileW - lipgloss.Width(line); pad > 0 {
			line += strings.Repeat(" ", pad)
		}
		b.WriteString("│" + line + "│\n")
	}
	b.WriteString("╰" + strings.Repeat("─", dashTileW) + "╯")
	return b.String()
}

func TestDashTileRunningByteStable(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	r := fleet.NodeResult{
		Name: "up", Outcome: fleet.OutcomeOK,
		Metrics: metrics.Stats{
			State: "running", Runner: "llamacpp", ModelID: "org/qwen:q4",
			UptimeSeconds: 7200, LastActiveAt: "2026-08-21T10:00:00Z", IdleSeconds: 12,
			CPU:    &metrics.CpuStat{Utilization: 42},
			Memory: &metrics.MemoryStat{Total: 1000, Used: 300},
			GPUs:   []metrics.GpuStat{{Index: 0, Name: "H100", Utilization: 61, MemoryUsed: 80, MemoryTotal: 160}},
			Tokens: &metrics.TokenStats{Running: 2, PromptTokens: 4096, GenerationTokens: 1024, Requests: 17},
		},
	}
	want := dashTileExpected([]string{
		dashHealthGlyph(dashHealthy) + " up  running  (up 2h 0m 0s)",
		"llamacpp  org/qwen:q4",
		"  last active 12s ago",
		dashBar("CPU", 42),
		dashBar("RAM", 30),
		dashBar("GPU util", 61),
		dashBar("GPU mem", 50),
		"",
		"  running:          2",
		"  prompt tokens:    4096",
		"  generation tokens: 1024",
		"  requests:         17",
	})
	if got := dashTile("up", r, false, dashAction{}); got != want {
		t.Errorf("tile mismatch:\ngot:\n%q\nwant:\n%q", got, want)
	}
}

func TestDashTileOutcomeAndEmpty(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	dead := fleet.NodeResult{
		Name: "down", Outcome: fleet.OutcomeUnreachable,
		Err: errors.New("connection refused (127.0.0.1:1)"),
	}
	if got := dashTile("down", dead, false, dashAction{}); got != dashTileExpected([]string{
		dashHealthGlyph(dashUnhealthy) + " down  unreachable",
		"connection refused (127.0.0.1:1)",
		"", "", "", "", "", "", "", "", "", "",
	}) {
		t.Errorf("outcome tile mismatch:\n%q", got)
	}
	// A node not answered yet is an empty panel naming the node.
	if got := dashTile("down", fleet.NodeResult{Name: "down"}, false, dashAction{}); got != dashTileExpected([]string{
		dashHealthGlyph(dashUnknown) + " down", "waiting for first refresh…",
		"", "", "", "", "", "", "", "", "", "",
	}) {
		t.Errorf("empty tile mismatch:\n%q", got)
	}
}

func TestDashTileStoppedByteStable(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	r := fleet.NodeResult{
		Name: "idle", Outcome: fleet.OutcomeOK,
		Metrics: metrics.Stats{State: "idle"},
	}
	want := dashTileExpected([]string{
		dashHealthGlyph(dashNotServing) + " idle  idle",
		"", "", "", "", "", "", "", "", "", "", "",
	})
	if got := dashTile("idle", r, false, dashAction{}); got != want {
		t.Errorf("stopped tile mismatch:\n%q\nwant:\n%q", got, want)
	}
}

// A node with an action in flight and no report yet shows the verb and the
// call's own lines; a stop conjugates: the p of stop drops before -ing.
func TestDashTileActionInFlight(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	if got := dashTile("dev-2", fleet.NodeResult{Name: "dev-2"}, false,
		dashAction{verb: "start", line: "instance starting; retrying in 42s"}); got != dashTileExpected([]string{
		dashHealthGlyph(dashAttention) + " dev-2  starting",
		"instance starting; retrying in 42s",
		"", "", "", "", "", "", "", "", "", "",
	}) {
		t.Errorf("in-flight tile mismatch:\n%q", got)
	}
	// A stop conjugates: the p of stop drops before -ing.
	if got := dashTile("dev-2", fleet.NodeResult{Name: "dev-2"}, false,
		dashAction{verb: "stop"}); got != dashTileExpected([]string{
		dashHealthGlyph(dashAttention) + " dev-2  stopping",
		"", "", "", "", "", "", "", "", "", "", "",
	}) {
		t.Errorf("bare in-flight tile mismatch:\n%q", got)
	}
}

// A report that lands while an action is in flight shows on the tile beside
// the call's own lines: the call says what the operator asked for, the report
// says what the node is doing — a boot half done already carries a state and
// whatever it measures, and that is the truth the tile should keep showing
// while the start still works.
func TestDashTileActionInFlightWithReport(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	// The instance is up and measuring, the engine not serving yet: the
	// start still works, and the report carries the state and the bars.
	r := fleet.NodeResult{
		Name: "vllm-1", Outcome: fleet.OutcomeOK,
		Metrics: metrics.Stats{
			State: "running", Runner: "vllm", ModelID: "org/qwen3:32b",
			UptimeSeconds: 240,
			CPU:           &metrics.CpuStat{Utilization: 12},
			Memory:        &metrics.MemoryStat{Total: 1000, Used: 480},
			GPUs:          []metrics.GpuStat{{Index: 0, Name: "H100", Utilization: 35, MemoryUsed: 72, MemoryTotal: 160}},
		},
	}
	want := dashTileExpected([]string{
		dashHealthGlyph(dashAttention) + " vllm-1  starting",
		"instance no-capacity; retrying in 120s",
		"running  (up 4m 0s)",
		"vllm  org/qwen3:32b",
		dashBar("CPU", 12),
		dashBar("RAM", 48),
		dashBar("GPU util", 35),
		dashBar("GPU mem", 45),
		"", "", "", "",
	})
	if got := dashTile("vllm-1", r, false,
		dashAction{verb: "start", line: "instance no-capacity; retrying in 120s"}); got != want {
		t.Errorf("in-flight tile with report mismatch:\ngot:\n%q\nwant:\n%q", got, want)
	}
	// Early in a boot: the report has a state and the serving, nothing
	// measured yet — the tile carries exactly that.
	early := fleet.NodeResult{
		Name: "vllm-1", Outcome: fleet.OutcomeOK,
		Metrics: metrics.Stats{State: "pending", Runner: "vllm", ModelID: "org/qwen3:32b"},
	}
	wantEarly := dashTileExpected([]string{
		dashHealthGlyph(dashAttention) + " vllm-1  starting",
		"instance starting; retrying in 60s",
		"pending",
		"vllm  org/qwen3:32b",
		"", "", "", "", "", "", "", "",
	})
	if got := dashTile("vllm-1", early, false,
		dashAction{verb: "start", line: "instance starting; retrying in 60s"}); got != wantEarly {
		t.Errorf("early-boot in-flight tile mismatch:\ngot:\n%q\nwant:\n%q", got, wantEarly)
	}
	// A round that failed this time says nothing on the tile: the call's own
	// lines stay the account, and the next round will say more.
	failed := fleet.NodeResult{
		Name: "vllm-1", Outcome: fleet.OutcomeUnreachable,
		Err: errors.New("stats returned HTTP 503: instance is not running"),
	}
	wantFailed := dashTileExpected([]string{
		dashHealthGlyph(dashAttention) + " vllm-1  starting",
		"instance starting; retrying in 60s",
		"", "", "", "", "", "", "", "", "", "",
	})
	if got := dashTile("vllm-1", failed, false,
		dashAction{verb: "start", line: "instance starting; retrying in 60s"}); got != wantFailed {
		t.Errorf("in-flight tile over a failed round mismatch:\ngot:\n%q\nwant:\n%q", got, wantFailed)
	}
}

func TestDashHealthTierFor(t *testing.T) {
	cases := []struct {
		name string
		r    fleet.NodeResult
		a    dashAction
		want dashHealthTier
	}{
		{"running and ready", fleet.NodeResult{Outcome: fleet.OutcomeOK,
			Metrics: metrics.Stats{State: "running", Ready: "ready"}}, dashAction{}, dashHealthy},
		{"running and not ready", fleet.NodeResult{Outcome: fleet.OutcomeOK,
			Metrics: metrics.Stats{State: "running", Ready: "not-ready"}}, dashAction{}, dashAttention},
		{"running with no readiness signal degrades to healthy", fleet.NodeResult{Outcome: fleet.OutcomeOK,
			Metrics: metrics.Stats{State: "running"}}, dashAction{}, dashHealthy},
		{"idle is not serving", fleet.NodeResult{Outcome: fleet.OutcomeOK,
			Metrics: metrics.Stats{State: "idle"}}, dashAction{}, dashNotServing},
		{"stopped is not serving", fleet.NodeResult{Outcome: fleet.OutcomeOK,
			Metrics: metrics.Stats{State: "stopped"}}, dashAction{}, dashNotServing},
		{"crashed is unhealthy", fleet.NodeResult{Outcome: fleet.OutcomeOK,
			Metrics: metrics.Stats{State: "crashed"}}, dashAction{}, dashUnhealthy},
		{"unreachable is unhealthy", fleet.NodeResult{Outcome: fleet.OutcomeUnreachable}, dashAction{}, dashUnhealthy},
		{"unauthorized is unhealthy", fleet.NodeResult{Outcome: fleet.OutcomeUnauthorized}, dashAction{}, dashUnhealthy},
		{"config error is unhealthy", fleet.NodeResult{Outcome: fleet.OutcomeConfigError}, dashAction{}, dashUnhealthy},
		{"failed is unhealthy", fleet.NodeResult{Outcome: fleet.OutcomeFailed}, dashAction{}, dashUnhealthy},
		{"unsupported is unhealthy", fleet.NodeResult{Outcome: fleet.OutcomeUnsupported}, dashAction{}, dashUnhealthy},
		{"no outcome yet is unknown", fleet.NodeResult{}, dashAction{}, dashUnknown},
		{"answered with no state is unknown", fleet.NodeResult{Outcome: fleet.OutcomeOK}, dashAction{}, dashUnknown},
		{"action in flight over a healthy report is attention regardless",
			fleet.NodeResult{Outcome: fleet.OutcomeOK, Metrics: metrics.Stats{State: "running", Ready: "ready"}},
			dashAction{verb: "start"}, dashAttention},
		{"action in flight over a crashed report is attention regardless",
			fleet.NodeResult{Outcome: fleet.OutcomeOK, Metrics: metrics.Stats{State: "crashed"}},
			dashAction{verb: "stop"}, dashAttention},
		{"start in flight over an idle report is attention, never not serving",
			fleet.NodeResult{Outcome: fleet.OutcomeOK, Metrics: metrics.Stats{State: "idle"}},
			dashAction{verb: "start"}, dashAttention},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := dashHealthTierFor(c.r, c.a); got != c.want {
				t.Errorf("dashHealthTierFor() = %v, want %v", got, c.want)
			}
		})
	}
}

// The selection is carried by the lit border; a colour profile that keeps
// colour is needed to see it, since the byte-stable profile strips it.
func TestDashTileSelectedBorderLit(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI256)
	r := fleet.NodeResult{Name: "n"}
	sel, unsel := dashTile("n", r, true, dashAction{}), dashTile("n", r, false, dashAction{})
	if !strings.Contains(sel, "\x1b[38;5;214m") {
		t.Errorf("selected tile carries no lit border:\n%q", sel)
	}
	if strings.Contains(unsel, "\x1b[38;5;214m") {
		t.Errorf("unselected tile carries the lit border:\n%q", unsel)
	}
	// The health glyph is unaffected by selection: same tier, same colour,
	// whether or not the border is lit.
	glyph := dashHealthGlyph(dashUnknown)
	if !strings.Contains(sel, glyph) {
		t.Errorf("selected tile's glyph changed:\n%q", sel)
	}
	if !strings.Contains(unsel, glyph) {
		t.Errorf("unselected tile's glyph changed:\n%q", unsel)
	}
}

// A line longer than the panel is hard-cut, never wrapped: the tile keeps
// its fixed shape whatever the model's id or a node's error message says.
func TestDashTileClipsLongLines(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	r := fleet.NodeResult{
		Name: "n", Outcome: fleet.OutcomeOK,
		Metrics: metrics.Stats{State: "running", ModelID: strings.Repeat("m", 60)},
	}
	rows := strings.Split(dashTile("n", r, false, dashAction{}), "\n")
	if len(rows) != dashTileH+2 {
		t.Fatalf("tile is %d rows, want %d", len(rows), dashTileH+2)
	}
	for i := 0; i < len(rows); i++ {
		if w := lipgloss.Width(rows[i]); w != dashTileW+2 {
			t.Errorf("row %d is %d columns, want %d", i, w, dashTileW+2)
		}
	}
}

// A long runner/model would once push uptime past dashClip's cutoff on the
// shared serving line; uptime now rides the state line instead, so it stays
// visible regardless of how long the serving line runs.
func TestDashTileUptimeSurvivesLongServingLine(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	r := fleet.NodeResult{
		Name: "n", Outcome: fleet.OutcomeOK,
		Metrics: metrics.Stats{
			State: "running", Runner: "vllm", ModelID: strings.Repeat("m", 40),
			UptimeSeconds: 125,
		},
	}
	lines := strings.Split(dashTile("n", r, false, dashAction{}), "\n")
	want := "│" + dashHealthGlyph(dashHealthy) + " n  running  (up 2m 5s)"
	if got := lines[1]; !strings.HasPrefix(got, want) {
		t.Errorf("state line = %q, want prefix %q", got, want)
	}
	if strings.Contains(lines[2], "(up ") {
		t.Errorf("uptime leaked onto the serving line: %q", lines[2])
	}
}

func TestDashStateLine(t *testing.T) {
	cases := []struct {
		name  string
		state string
		up    int
		want  string
	}{
		{"no uptime keeps state alone", "running", 0, "running"},
		{"uptime pairs with state", "running", 125, "running  (up 2m 5s)"},
		// State is empty in practice only for a node with no report at all,
		// which also carries no uptime — but the helper still has to make a
		// sane choice rather than lead with a stray double space.
		{"uptime with no state has no leading gap", "", 125, "(up 2m 5s)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := dashStateLine(c.state, metrics.Stats{UptimeSeconds: c.up})
			if got != c.want {
				t.Errorf("dashStateLine(%q, uptime=%d) = %q, want %q", c.state, c.up, got, c.want)
			}
		})
	}
}

// More GPUs than the tile holds: the content is cut to the tile height, not
// wrapped — a one-GPU node fills the tile exactly, four GPUs may not.
func TestDashTileTruncatesTallContent(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	gpus := make([]metrics.GpuStat, 4)
	for i := range gpus {
		gpus[i] = metrics.GpuStat{Index: i, Name: "H100", Utilization: 10 * (i + 1), MemoryUsed: 1, MemoryTotal: 10}
	}
	r := fleet.NodeResult{
		Name: "many", Outcome: fleet.OutcomeOK,
		Metrics: metrics.Stats{
			State: "running", Runner: "llamacpp", ModelID: "org/qwen",
			UptimeSeconds: 60,
			CPU:           &metrics.CpuStat{Utilization: 42},
			Memory:        &metrics.MemoryStat{Total: 1000, Used: 300},
			GPUs:          gpus,
			Tokens:        &metrics.TokenStats{Running: 1, PromptTokens: 100, GenerationTokens: 50, Requests: 3},
		},
	}
	lines := strings.Split(dashTile("many", r, false, dashAction{}), "\n")
	if len(lines) != dashTileH+2 {
		t.Errorf("a tall node broke the tile geometry: %d lines (want %d)", len(lines), dashTileH+2)
	}
}

// The scroll row never scrolls past an end: a small fleet in a big terminal
// cannot scroll at all, and a deep scroll clamps to the last row.
func TestDashClampScrollBounds(t *testing.T) {
	if got := dashClampScroll(5, 2, 120, 40); got != 0 {
		t.Errorf("a two-node fleet scrolls: %d (want 0)", got)
	}
	if got := dashClampScroll(0, 100, 120, 40); got != 0 {
		t.Errorf("fresh scroll rows move: %d (want 0)", got)
	}
	// 100 nodes in four columns of two is fifty rows; the last visible window
	// starts at row forty-eight.
	if got := dashClampScroll(99, 100, 120, 40); got != 48 {
		t.Errorf("deep scroll: %d (want 48)", got)
	}
}

func TestDashLayoutMath(t *testing.T) {
	cols := []struct {
		w, want int
	}{
		{10, 1}, {44, 1}, {45, 1}, {88, 1},
		{89, 2}, {80, 1},
		{134, 3}, {133, 2}, {179, 4}, {178, 3},
	}
	for _, c := range cols {
		if got := dashCols(c.w); got != c.want {
			t.Errorf("dashCols(%d) = %d, want %d", c.w, got, c.want)
		}
	}
	rows := []struct {
		h, want int
	}{
		{3, 1}, {17, 1}, {18, 1}, {32, 2}, {47, 3}, {46, 2},
	}
	for _, c := range rows {
		if got := dashVisibleRows(c.h); got != c.want {
			t.Errorf("dashVisibleRows(%d) = %d, want %d", c.h, got, c.want)
		}
	}
	// Ten nodes, two columns (w=133), two visible rows (h=32): five grid
	// rows, so the top may scroll to row 3.
	if got := dashClampScroll(0, 10, 133, 32); got != 0 {
		t.Errorf("clamp 0 = %d", got)
	}
	if got := dashClampScroll(3, 10, 133, 32); got != 3 {
		t.Errorf("clamp 3 = %d", got)
	}
	if got := dashClampScroll(9, 10, 133, 32); got != 3 {
		t.Errorf("clamp 9 = %d, want 3", got)
	}
	if got := dashClampScroll(-4, 10, 133, 32); got != 0 {
		t.Errorf("clamp -4 = %d", got)
	}
	// Fewer tiles than a screen: nowhere to scroll to.
	if got := dashClampScroll(5, 3, 133, 32); got != 0 {
		t.Errorf("small-grid clamp = %d, want 0", got)
	}
	// Grid fill is left to right, top to bottom, in file order.
	lines := []string{"t0", "t1", "t2", "t3", "t4"}
	got := dashGridRows(lines, 2)
	want := []string{"t0 t1", "t2 t3", "t4"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Errorf("dashGridRows = %q, want %q", got, want)
	}
	// A real tile is a multi-line block, so a grid row joins the tiles'
	// corresponding lines: the second tile's top border sits beside the
	// first tile's, never by its bottom.
	block := func(tag byte) string {
		parts := make([]string, dashTileH+2)
		for l := range parts {
			parts[l] = fmt.Sprintf("%c%02d", tag, l)
		}
		return strings.Join(parts, "\n")
	}
	got = dashGridRows([]string{block('A'), block('B')}, 2)
	if len(got) != 1 {
		t.Fatalf("two blocks in two columns make %d rows, want 1", len(got))
	}
	top, bottom := strings.Split(got[0], "\n")[0], strings.Split(got[0], "\n")[dashTileH+1]
	if len(strings.Split(got[0], "\n")) != dashTileH+2 {
		t.Fatalf("the joined row has %d lines, want %d", len(strings.Split(got[0], "\n")), dashTileH+2)
	}
	if top != "A00 B00" || bottom != fmt.Sprintf("A%02d B%02d", dashTileH+1, dashTileH+1) {
		t.Errorf("block rows misjoined: top %q, bottom %q", top, bottom)
	}
	// A partial row: the lone tile stands in its own column.
	got = dashGridRows([]string{block('A'), block('B'), block('C')}, 2)
	if len(got) != 2 || len(strings.Split(got[0], "\n")) != dashTileH+2 || got[1] != block('C') {
		t.Errorf("partial row: %d rows, %q", len(got), got)
	}
}

// Three real tiles at a wide window must sit side by side — the user's
// original report: the second and third panels' top borders appeared beside
// the first panel's bottom border.
func TestDashGridRealTilesSideBySide(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	running := func(name string) fleet.NodeResult {
		return fleet.NodeResult{
			Name: name, Outcome: fleet.OutcomeOK,
			Metrics: metrics.Stats{
				State: "running", Runner: "llamacpp", ModelID: "unsloth/qwen", UptimeSeconds: 2820,
				LastActiveAt: "2026-08-22T10:00:00Z", IdleSeconds: 29,
				CPU:    &metrics.CpuStat{Utilization: 1},
				Memory: &metrics.MemoryStat{Total: 100, Used: 35},
				GPUs:   []metrics.GpuStat{{Index: 0, Name: "H100", Utilization: 61, MemoryUsed: 89, MemoryTotal: 100}},
				Tokens: &metrics.TokenStats{Running: 1, PromptTokens: 324040, GenerationTokens: 35525},
			},
		}
	}
	m := &dashModel{
		fleetPath: "fleet.yaml",
		entries: []dashEntry{
			{name: "dev-1", kind: fleet.KindDaemon},
			{name: "dev-2", kind: fleet.KindDaemon},
			{name: "dev-3", kind: fleet.KindDaemon},
		},
		results: []fleet.NodeResult{running("dev-1"), running("dev-2"), running("dev-3")},
		actions: make([]dashAction, 3),
		width:   200, height: 40,
	}
	lines := strings.Split(m.View(), "\n")
	// Header, one grid row of 14 lines, footer: nothing extra, nothing short.
	if len(lines) != 2+dashTileH+2 {
		t.Fatalf("board is %d lines, want %d", len(lines), 2+dashTileH+2)
	}
	top, bottom := lines[1], lines[1+dashTileH+1]
	if strings.Count(top, "╭") != 3 || strings.Count(bottom, "╯") != 3 {
		t.Errorf("borders split across lines:\ntop:    %q\nbottom: %q", top, bottom)
	}
	// The frame is drawn from box-drawing runes (three bytes each), so the
	// position is counted in runes, the board's own display columns: the
	// second tile starts one step right of the first tile's left edge.
	col, seen := -1, 0
	for i, r := range []rune(top) {
		if r != '╭' {
			continue
		}
		seen++
		if seen == 2 {
			col = i
			break
		}
	}
	if col != dashTileStep {
		t.Errorf("second tile's top border at column %d, want %d:\n%s", col, dashTileStep, top)
	}
	// A body line carries every tile's wall at once.
	if strings.Count(lines[2], "│") != 6 {
		t.Errorf("body line missing a tile's walls:\n%q", lines[2])
	}
}

// Selection moves and scrolls keep the chosen tile in the visible rows.
func TestDashModelSelectionScroll(t *testing.T) {
	entries := make([]dashEntry, 6)
	results := make([]fleet.NodeResult, 6)
	for i := range entries {
		entries[i] = dashEntry{name: fmt.Sprintf("n%d", i)}
		results[i] = fleet.NodeResult{Name: entries[i].name}
	}
	m := &dashModel{entries: entries, results: results, actions: make([]dashAction, len(entries)), width: 120, height: 40}
	// Two columns, two visible rows: down moves by a full grid row (±2), not
	// by one flat index, so a single press already skips to the next row.
	next, cmd := m.Update(dashKey("down"))
	if cmd != nil {
		t.Fatalf("cursor move returned a cmd")
	}
	m = next.(*dashModel)
	if got := m.cursor; got != 2 {
		t.Fatalf("cursor after one down = %d, want 2 (a row move, not +1)", got)
	}
	// A second row-down lands on node 4 (grid row 2), which needs scrolling.
	next, cmd = m.Update(dashKey("down"))
	if cmd != nil {
		t.Fatalf("cursor move returned a cmd")
	}
	m = next.(*dashModel)
	if got := m.cursor; got != 4 {
		t.Fatalf("cursor = %d, want 4", got)
	}
	// The visible rows are 0-1, so row 2 shows as the lower one: the top
	// sits at row 1.
	if got := m.scrollRow; got != 1 {
		t.Errorf("scrollRow = %d, want 1", got)
	}
	// Paging stays within the grid: up three times lands on the top row, and
	// the fourth up is a no-op.
	for i := 0; i < 3; i++ {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
		m = next.(*dashModel)
	}
	if got := m.scrollRow; got != 0 {
		t.Errorf("page up = %d, want 0", got)
	}
	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyPgUp}); cmd != nil {
		t.Errorf("page up at the top returned a cmd")
	}
}

// Left and right move within a row only, and clamp rather than wrap at
// either end of it.
func TestDashModelLeftRightNavigation(t *testing.T) {
	entries := make([]dashEntry, 6)
	results := make([]fleet.NodeResult, 6)
	for i := range entries {
		entries[i] = dashEntry{name: fmt.Sprintf("n%d", i)}
		results[i] = fleet.NodeResult{Name: entries[i].name}
	}
	// Two columns: row 0 is nodes 0-1.
	m := &dashModel{entries: entries, results: results, actions: make([]dashAction, len(entries)), width: 120, height: 40}

	next, cmd := m.Update(dashKey("right"))
	if cmd != nil {
		t.Fatalf("cursor move returned a cmd")
	}
	m = next.(*dashModel)
	if got := m.cursor; got != 1 {
		t.Fatalf("cursor after right = %d, want 1", got)
	}

	next, _ = m.Update(dashKey("right"))
	m = next.(*dashModel)
	if got := m.cursor; got != 1 {
		t.Fatalf("right at the row's last column moved the selection: %d", got)
	}

	next, _ = m.Update(dashKey("left"))
	m = next.(*dashModel)
	if got := m.cursor; got != 0 {
		t.Fatalf("cursor after left = %d, want 0", got)
	}

	next, _ = m.Update(dashKey("left"))
	m = next.(*dashModel)
	if got := m.cursor; got != 0 {
		t.Fatalf("left at the row's first column moved the selection: %d", got)
	}
}

// A short last row (fewer tiles than the column count) clamps down to the
// last entry that exists, rather than moving past the end of the fleet.
func TestDashModelDownClampsShortLastRow(t *testing.T) {
	entries := make([]dashEntry, 5)
	results := make([]fleet.NodeResult, 5)
	for i := range entries {
		entries[i] = dashEntry{name: fmt.Sprintf("n%d", i)}
		results[i] = fleet.NodeResult{Name: entries[i].name}
	}
	// Two columns, five nodes: row 0 = [0,1], row 1 = [2,3], row 2 = [4] alone.
	m := &dashModel{entries: entries, results: results, actions: make([]dashAction, len(entries)), width: 120, height: 40, cursor: 3}

	next, _ := m.Update(dashKey("down"))
	m = next.(*dashModel)
	if got := m.cursor; got != 4 {
		t.Fatalf("cursor after down from a short-row column = %d, want 4", got)
	}

	// Already on the last entry: another down is a no-op, not a move past it.
	next, _ = m.Update(dashKey("down"))
	m = next.(*dashModel)
	if got := m.cursor; got != 4 {
		t.Fatalf("down past the last entry moved the selection: %d", got)
	}
}

// Right on a short last row must not step into a column that fits the grid
// width but has no tile in it — the "column exists, tile doesn't" case,
// distinct from the row-clamping down already covers.
func TestDashModelRightClampsShortLastRow(t *testing.T) {
	entries := make([]dashEntry, 5)
	results := make([]fleet.NodeResult, 5)
	for i := range entries {
		entries[i] = dashEntry{name: fmt.Sprintf("n%d", i)}
		results[i] = fleet.NodeResult{Name: entries[i].name}
	}
	// Two columns, five nodes: row 2 holds only node 4, at column 0 — column
	// 1 of that row fits the grid's width but has nothing in it.
	m := &dashModel{entries: entries, results: results, actions: make([]dashAction, len(entries)), width: 120, height: 40, cursor: 4}

	next, _ := m.Update(dashKey("right"))
	m = next.(*dashModel)
	if got := m.cursor; got != 4 {
		t.Fatalf("right into an empty grid cell moved the selection: %d", got)
	}
}

// A single-column layout (a narrow terminal) has no adjacent column at all:
// left and right must be no-ops everywhere, not just at the row's ends.
func TestDashModelLeftRightNoOpInSingleColumn(t *testing.T) {
	entries := make([]dashEntry, 3)
	results := make([]fleet.NodeResult, 3)
	for i := range entries {
		entries[i] = dashEntry{name: fmt.Sprintf("n%d", i)}
		results[i] = fleet.NodeResult{Name: entries[i].name}
	}
	// 80 columns wide fits one tile per row (dashCols(80) == 1).
	m := &dashModel{entries: entries, results: results, actions: make([]dashAction, len(entries)), width: 80, height: 40, cursor: 1}
	if got := dashCols(m.effWidth()); got != 1 {
		t.Fatalf("fixture assumption wrong: dashCols(80) = %d, want 1", got)
	}

	next, _ := m.Update(dashKey("right"))
	m = next.(*dashModel)
	if got := m.cursor; got != 1 {
		t.Fatalf("right in a single-column grid moved the selection: %d", got)
	}

	next, _ = m.Update(dashKey("left"))
	m = next.(*dashModel)
	if got := m.cursor; got != 1 {
		t.Fatalf("left in a single-column grid moved the selection: %d", got)
	}
}

// Pressing the up key itself — not pgup — must scroll the view back into
// place when the move lands above what is currently visible.
func TestDashModelUpScrollsView(t *testing.T) {
	entries := make([]dashEntry, 6)
	results := make([]fleet.NodeResult, 6)
	for i := range entries {
		entries[i] = dashEntry{name: fmt.Sprintf("n%d", i)}
		results[i] = fleet.NodeResult{Name: entries[i].name}
	}
	// Two columns, two visible rows: start on the bottom row (scrolled down),
	// then move up twice back to the top row.
	m := &dashModel{entries: entries, results: results, actions: make([]dashAction, len(entries)), width: 120, height: 40, cursor: 4}
	m.keepVisible()
	if got := m.scrollRow; got != 1 {
		t.Fatalf("fixture assumption wrong: scrollRow = %d, want 1", got)
	}

	next, _ := m.Update(dashKey("up"))
	m = next.(*dashModel)
	if got := m.cursor; got != 2 {
		t.Fatalf("cursor after up = %d, want 2", got)
	}

	next, _ = m.Update(dashKey("up"))
	m = next.(*dashModel)
	if got := m.cursor; got != 0 {
		t.Fatalf("cursor after second up = %d, want 0", got)
	}
	if got := m.scrollRow; got != 0 {
		t.Errorf("up did not scroll the view back to the top row: scrollRow = %d, want 0", got)
	}
}

// Up on the top row must never move the selection sideways: with no row
// above to land on, it is a no-op, whatever column the cursor is in — not a
// jump to index 0, which would drag a non-leftmost column back to column 0.
func TestDashModelUpNoOpOnTopRow(t *testing.T) {
	entries := make([]dashEntry, 4)
	results := make([]fleet.NodeResult, 4)
	for i := range entries {
		entries[i] = dashEntry{name: fmt.Sprintf("n%d", i)}
		results[i] = fleet.NodeResult{Name: entries[i].name}
	}
	// Two columns: node 1 is the top row's second (rightmost) column.
	m := &dashModel{entries: entries, results: results, actions: make([]dashAction, len(entries)), width: 120, height: 40, cursor: 1}

	next, _ := m.Update(dashKey("up"))
	m = next.(*dashModel)
	if got := m.cursor; got != 1 {
		t.Fatalf("up on the top row moved the selection sideways to %d, want 1", got)
	}
}

// Down while already on the grid's last row must never move the selection
// sideways: with no row below to land on, it is a no-op, whatever column
// the cursor is in — not a jump to the last entry, which would drag the
// leftmost column of a short last row over to its rightmost one.
func TestDashModelDownNoOpOnLastRow(t *testing.T) {
	entries := make([]dashEntry, 5)
	results := make([]fleet.NodeResult, 5)
	for i := range entries {
		entries[i] = dashEntry{name: fmt.Sprintf("n%d", i)}
		results[i] = fleet.NodeResult{Name: entries[i].name}
	}
	// Three columns, five nodes: row 0 = [0,1,2], row 1 = [3,4] — a short
	// last row. Node 3 is already on that last row, at its first column.
	m := &dashModel{entries: entries, results: results, actions: make([]dashAction, len(entries)), width: 150, height: 40, cursor: 3}
	if got := dashCols(m.effWidth()); got != 3 {
		t.Fatalf("fixture assumption wrong: dashCols(150) = %d, want 3", got)
	}

	next, _ := m.Update(dashKey("down"))
	m = next.(*dashModel)
	if got := m.cursor; got != 3 {
		t.Fatalf("down on the last row moved the selection sideways to %d, want 3", got)
	}
}

// A resize that changes the column count is picked up by the very next
// move — selection follows the grid currently drawn, not a stale one.
func TestDashModelResizeChangesGrid(t *testing.T) {
	entries := make([]dashEntry, 6)
	results := make([]fleet.NodeResult, 6)
	for i := range entries {
		entries[i] = dashEntry{name: fmt.Sprintf("n%d", i)}
		results[i] = fleet.NodeResult{Name: entries[i].name}
	}
	// Starts at 120 columns wide (2 tiles per row).
	m := &dashModel{entries: entries, results: results, actions: make([]dashAction, len(entries)), width: 120, height: 40}

	next, _ := m.Update(dashKey("right"))
	m = next.(*dashModel)
	if got := m.cursor; got != 1 {
		t.Fatalf("cursor after right = %d, want 1", got)
	}

	// Widen to 150 columns (3 tiles per row) before the next move.
	next, _ = m.Update(tea.WindowSizeMsg{Width: 150, Height: 40})
	m = next.(*dashModel)
	if got := dashCols(m.effWidth()); got != 3 {
		t.Fatalf("fixture assumption wrong: dashCols(150) = %d, want 3", got)
	}

	next, _ = m.Update(dashKey("down"))
	m = next.(*dashModel)
	if got := m.cursor; got != 4 {
		t.Fatalf("cursor after down post-resize = %d, want 4 (3-column grid, not the old 2-column one)", got)
	}
}

// A tick while a group's round is in flight is absorbed; a round's answer
// lands by generation, and a superseded answer is dropped rather than
// painted over the board.
func TestDashModelRefreshGenerations(t *testing.T) {
	node := newFakeDashNode("running")
	m := &dashModel{
		entries: []dashEntry{{name: "a", kind: fleet.KindDaemon, node: node}},
		results: []fleet.NodeResult{{Name: "a"}},
		actions: make([]dashAction, 1),
		width:   120, height: 40,
	}
	// In flight: the tick reschedules itself but must not begin a second
	// round in the group.
	m.fastBusy = true
	m.fastGen = 7
	m2, _ := m.Update(dashTickMsg{})
	if m2.(*dashModel).fastGen != 7 {
		t.Fatal("tick while refreshing started another round")
	}
	m = m2.(*dashModel)
	// A stale answer: discarded.
	stale := []fleet.NodeResult{{Outcome: fleet.OutcomeOK}}
	fresh := []fleet.NodeResult{{Outcome: fleet.OutcomeUnreachable}}
	m2, _ = m.Update(dashRefreshMsg{gen: 6, idx: []int{0}, results: stale})
	if m2.(*dashModel).results[0].Outcome != "" {
		t.Fatalf("stale round painted the board: %v", m2.(*dashModel).results[0].Outcome)
	}
	m3, _ := m2.(*dashModel).Update(dashRefreshMsg{gen: 7, idx: []int{0}, results: fresh})
	mm := m3.(*dashModel)
	if mm.fastBusy {
		t.Fatal("round completion did not clear the flag")
	}
	if mm.results[0].Outcome != fleet.OutcomeUnreachable {
		t.Fatalf("current round not applied: %v", mm.results[0].Outcome)
	}
	// A live round: per-node answers, in entry order.
	msg, ok := startFastRound(t, mm)
	if !ok {
		t.Fatal("no round began")
	}
	if !msg.results[0].OK() || msg.results[0].Metrics.State != "running" {
		t.Errorf("live node answer: %+v", msg.results[0])
	}
}

// A kind: remote environment refreshes on its own slower deadline: the round
// starts only when that time comes, starting it spends the deadline, and the
// manual refresh brings it forward whatever the deadline says.
func TestDashModelRemoteCadence(t *testing.T) {
	orig := dashboardRemoteRefreshInterval
	dashboardRemoteRefreshInterval = time.Minute
	defer func() { dashboardRemoteRefreshInterval = orig }()

	local := newFakeDashNode("running")
	r1 := newFakeDashNode("running")
	r2 := newFakeDashNode("running")
	m := &dashModel{
		entries: []dashEntry{
			{name: "local", kind: fleet.KindDaemon, node: local},
			{name: "r1", kind: fleet.KindRemote, node: r1},
			{name: "r2", kind: fleet.KindRemote, node: r2},
		},
		results: make([]fleet.NodeResult, 3),
		actions: make([]dashAction, 3),
		width:   120, height: 40,
	}
	// Cold open: the local round and the cloud round are both due — the
	// cloud deadline has never been spent.
	cmds := m.startRounds()
	if len(cmds) != 2 {
		t.Fatalf("cold open started %d rounds, want one per group", len(cmds))
	}
	m = landRounds(t, m, cmds)
	for i := range m.entries {
		if !m.results[i].OK() || m.results[i].Metrics.State != "running" {
			t.Errorf("cold round missed %s: %+v", m.entries[i].name, m.results[i])
		}
	}
	if m.fastBusy || m.slowBusy {
		t.Fatal("rounds still marked in flight after their answers landed")
	}
	if !m.nextSlowAt.After(time.Now()) {
		t.Fatal("starting the cloud round did not spend its deadline")
	}
	// Before the deadline, a tick is due for the local round only.
	cmds = m.startRounds()
	if len(cmds) != 1 {
		t.Fatalf("early tick started %d rounds, want the local one", len(cmds))
	}
	m = landRounds(t, m, cmds)
	// At the deadline, both groups are due again.
	m.nextSlowAt = time.Now().Add(-time.Millisecond)
	if cmds = m.startRounds(); len(cmds) != 2 {
		t.Fatalf("due tick started %d rounds, want both groups", len(cmds))
	}
	m = landRounds(t, m, cmds)
	// A manual refresh is due for every node, whatever the deadline says.
	m.nextSlowAt = time.Now().Add(time.Hour)
	m2, _ := m.Update(dashKey("r"))
	mm := m2.(*dashModel)
	if mm.nextSlowAt.IsZero() {
		t.Error("the manual refresh did not bring the cloud deadline forward")
	}
	if !mm.fastBusy || !mm.slowBusy {
		t.Errorf("the manual refresh did not start both groups: fastBusy=%v slowBusy=%v", mm.fastBusy, mm.slowBusy)
	}
}

// One dead node does not keep its neighbour's answer waiting: each node
// answers on its own, and the fan-out calls them concurrently.
func TestDashRoundDoesNotWaitOnDeadNode(t *testing.T) {
	up := newFakeDashNode("running")
	down := newFakeDashNode("stopped")
	down.failMetrics(errors.New("connection refused (127.0.0.1:9)"))
	m := &dashModel{
		entries: []dashEntry{
			{name: "up", kind: fleet.KindDaemon, node: up},
			{name: "down", kind: fleet.KindDaemon, node: down},
		},
		results: make([]fleet.NodeResult, 2),
		actions: make([]dashAction, 2),
		width:   120, height: 40,
	}
	msg, ok := startFastRound(t, m)
	if !ok {
		t.Fatal("round did not run")
	}
	if !msg.results[0].OK() {
		t.Errorf("up node: %+v", msg.results[0])
	}
	if msg.results[1].Outcome != fleet.OutcomeUnreachable {
		t.Errorf("down node: %+v", msg.results[1])
	}
}

// s starts; x asks, n declines, y confirms. A node that already has an
// action in flight takes no more starts, and a finished start clears the
// node so a stop can follow.
func TestDashModelStartAndStop(t *testing.T) {
	node := newFakeDashNode("stopped")
	m := &dashModel{
		entries: []dashEntry{{name: "a", kind: fleet.KindDaemon, node: node}},
		results: []fleet.NodeResult{{Name: "a"}},
		actions: make([]dashAction, 1),
		width:   120, height: 40,
	}
	m2, cmd := m.Update(dashKey("s"))
	if cmd == nil {
		t.Fatal("s did not start")
	}
	mm := m2.(*dashModel)
	if mm.actions[0].verb != "start" {
		t.Fatalf("no action recorded on the node: %+v", mm.actions[0])
	}
	// In flight: no second start on the same node.
	if _, cmd2 := mm.Update(dashKey("s")); cmd2 != nil {
		t.Fatal("a node with an action in flight started again")
	}
	smsg, _ := cmd().(dashActionMsg)
	if node.starts != 1 {
		t.Fatal("start not called")
	}
	m3, _ := mm.Update(smsg)
	mm = m3.(*dashModel)
	if mm.actions[0].verb != "" {
		t.Fatalf("the finished action was not cleared: %+v", mm.actions[0])
	}
	if !strings.Contains(mm.statusLine, "a: start — running") {
		t.Errorf("start status line: %q", mm.statusLine)
	}
	// Decline: no call.
	m4, _ := mm.Update(dashKey("x"))
	if !m4.(*dashModel).confirm {
		t.Fatal("x did not confirm")
	}
	m5, _ := m4.(*dashModel).Update(dashKey("n"))
	if m5.(*dashModel).confirm {
		t.Fatal("n did not decline")
	}
	if node.stops != 0 {
		t.Fatal("stop called on a declined confirmation")
	}
	// Confirm: the stop flies.
	m6, _ := m5.(*dashModel).Update(dashKey("x"))
	m7, cmd2 := m6.(*dashModel).Update(dashKey("y"))
	if cmd2 == nil {
		t.Fatal("y did not stop")
	}
	pmsg, _ := cmd2().(dashActionMsg)
	if node.stops != 1 {
		t.Fatal("stop not called")
	}
	m8, _ := m7.Update(pmsg)
	pmm := m8.(*dashModel)
	if pmm.confirm {
		t.Fatal("confirmation flag stuck after the stop")
	}
	if !strings.Contains(pmm.statusLine, "a: stop — stopped") {
		t.Errorf("stop status line: %q", pmm.statusLine)
	}
}

// An action on an entry that could not build a node reports the standing
// reason and calls nothing.
func TestDashModelActionOnStandingNode(t *testing.T) {
	m := &dashModel{
		entries: []dashEntry{{
			name:     "broken",
			kind:     fleet.KindDaemon,
			standing: fleet.NodeResult{Name: "broken", Outcome: fleet.OutcomeConfigError, Err: errors.New(`tokenEnv "NOPE" is set nowhere`)},
		}},
		results: []fleet.NodeResult{{Name: "broken", Outcome: fleet.OutcomeConfigError, Err: errors.New(`tokenEnv "NOPE" is set nowhere`)}},
		actions: make([]dashAction, 1),
		width:   120, height: 40,
	}
	m2, cmd := m.Update(dashKey("s"))
	if cmd != nil {
		t.Fatal("nothing to call on an unbuilt node")
	}
	if !strings.Contains(m2.(*dashModel).statusLine, "set nowhere") {
		t.Errorf("status line: %q", m2.(*dashModel).statusLine)
	}
}

// Two nodes start at the same time: the action is one per node, in-flight
// progress lands on the node's own tile, and each final clears its own node.
func TestDashModelConcurrentStarts(t *testing.T) {
	a := newFakeDashNode("stopped")
	b := newFakeDashNode("stopped")
	m := &dashModel{
		entries: []dashEntry{
			{name: "a", kind: fleet.KindRemote, node: a},
			{name: "b", kind: fleet.KindRemote, node: b},
		},
		results: make([]fleet.NodeResult, 2),
		actions: make([]dashAction, 2),
		width:   120, height: 40,
	}
	var caught []tea.Msg
	m.send = func(msg tea.Msg) { caught = append(caught, msg) }

	next, cmd := m.Update(dashKey("s"))
	if cmd == nil {
		t.Fatal("s did not start node a")
	}
	m = next.(*dashModel)
	if m.actions[0].verb != "start" {
		t.Fatalf("node a's action not recorded: %+v", m.actions[0])
	}
	// In flight: node a takes no more starts…
	if _, cmd2 := m.Update(dashKey("s")); cmd2 != nil {
		t.Fatal("node a started again while its start was in flight")
	}
	// …but node b starts too. Two columns, two nodes: b sits beside a in the
	// same row, so right (not down) reaches it.
	next, _ = m.Update(dashKey("right"))
	m = next.(*dashModel)
	next, cmdB := m.Update(dashKey("s"))
	if cmdB == nil {
		t.Fatal("node b did not start while node a was in flight")
	}
	m = next.(*dashModel)
	if m.actions[0].verb != "start" || m.actions[1].verb != "start" {
		t.Fatalf("both actions should be in flight: %+v", m.actions)
	}
	if v := m.View(); !strings.Contains(v, "a  starting") || !strings.Contains(v, "b  starting") {
		t.Errorf("the in-flight tiles do not carry the verb:\n%s", v)
	}
	// Run both calls: each reports its own line through the send door.
	msgA := cmd()
	msgB := cmdB()
	for _, msg := range caught {
		next, _ = m.Update(msg)
		m = next.(*dashModel)
	}
	if m.actions[0].line != "instance starting; retrying in 1s" ||
		m.actions[1].line != "instance starting; retrying in 1s" {
		t.Errorf("progress lines not on the nodes: %+v", m.actions)
	}
	if v := m.View(); !strings.Contains(v, "retrying in 1s") {
		t.Errorf("the tile does not show the call's line:\n%s", v)
	}
	// Each final clears its own node and leaves its line in the footer.
	next, _ = m.Update(msgA)
	m = next.(*dashModel)
	if m.actions[0].verb != "" {
		t.Fatalf("node a's final did not clear it: %+v", m.actions[0])
	}
	if m.actions[1].verb != "start" {
		t.Fatalf("node a's final touched node b: %+v", m.actions[1])
	}
	if !strings.Contains(m.statusLine, "a: start — running") {
		t.Errorf("status line: %q", m.statusLine)
	}
	next, _ = m.Update(msgB)
	m = next.(*dashModel)
	if m.actions[1].verb != "" || !strings.Contains(m.statusLine, "b: start — running") {
		t.Errorf("node b's final: %+v, line %q", m.actions, m.statusLine)
	}
}

// A round that lands while a start is in flight paints on the tile beside
// the call's own lines: the node's report and the call's account both show,
// until the call returns and the report stands alone.
func TestDashModelLandedRoundShowsBesideInFlightAction(t *testing.T) {
	f := newFakeDashNode("stopped")
	m := &dashModel{
		entries: []dashEntry{{name: "a", kind: fleet.KindRemote, node: f}},
		results: make([]fleet.NodeResult, 1),
		actions: make([]dashAction, 1),
		width:   120, height: 40,
	}
	next, _ := m.Update(dashKey("s"))
	m = next.(*dashModel)
	if m.actions[0].verb != "start" {
		t.Fatalf("no action recorded: %+v", m.actions[0])
	}
	// The call's own line, as its goroutine would send it.
	next, _ = m.Update(dashActionProgressMsg{node: "a", line: "instance starting; retrying in 1s"})
	m = next.(*dashModel)
	// The cloud round lands while the start is in flight.
	cmd := m.refreshRemoteGroup(true)
	if cmd == nil {
		t.Fatal("the cloud round did not start")
	}
	msg, _ := cmd().(dashRefreshMsg)
	next, _ = m.Update(msg)
	m = next.(*dashModel)
	v := m.View()
	for _, want := range []string{
		"a  starting", "instance starting; retrying in 1s", "stopped", "llamacpp  org/qwen",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("the in-flight tile does not carry %q:\n%s", want, v)
		}
	}
}

// Up holds at the top, k backtracks to it without going negative, pgdown
// holds at the bottom, and a stale scroll row past the grid is bounded when
// the frame is drawn.
func TestDashModelUpNavigationClamps(t *testing.T) {
	entries := make([]dashEntry, 7)
	for i := range entries {
		entries[i] = dashEntry{name: fmt.Sprintf("n%d", i), kind: fleet.KindDaemon, node: newFakeDashNode("running")}
	}
	m := &dashModel{
		entries: entries,
		results: make([]fleet.NodeResult, 7),
		actions: make([]dashAction, 7),
		width:   120, height: 40,
	}
	step := func(key string) *dashModel {
		next, _ := m.Update(dashKey(key))
		m = next.(*dashModel)
		return m
	}
	if step("up").cursor != 0 {
		t.Fatal("up at the top moved the selection")
	}
	// Two columns: each down moves by a row (±2), not a flat +1.
	if got := step("down").cursor; got != 2 {
		t.Fatalf("cursor after one down = %d, want 2", got)
	}
	for i := 1; i < len(entries)-1; i++ {
		step("down")
	}
	if m.cursor != 6 {
		t.Fatalf("selection not on the last node: %d", m.cursor)
	}
	for i := 0; i < len(entries); i++ { // one more k than there are nodes
		step("up")
	}
	if m.cursor != 0 {
		t.Fatalf("k past the top: %d", m.cursor)
	}
	m.cursor = len(entries) - 1
	m.keepVisible()
	before := m.scrollRow
	if step("pgdown").scrollRow != before {
		t.Errorf("pgdown past the bottom moved the scroll row: %d", m.scrollRow)
	}
	// A stale scroll row past the grid is bounded: the frame still draws
	// rather than panicking, and stays put until the next move re-scrolls.
	m.scrollRow = 99
	v := m.View()
	if !strings.Contains(v, "fleet dashboard") || strings.Contains(v, "n0") {
		t.Errorf("a stale scroll row broke the frame:\n%s", v)
	}
}

// q in the middle of a stop confirmation sends nothing and leaves.
func TestDashModelQuitDuringConfirmation(t *testing.T) {
	node := newFakeDashNode("running")
	m := &dashModel{
		entries: []dashEntry{{name: "a", kind: fleet.KindDaemon, node: node}},
		results: []fleet.NodeResult{{Name: "a"}},
		actions: make([]dashAction, 1),
		width:   120, height: 40,
	}
	next, _ := m.Update(dashKey("x"))
	m = next.(*dashModel)
	if !m.confirm {
		t.Fatal("x did not open the confirmation")
	}
	next, cmd := m.Update(dashKey("q"))
	m = next.(*dashModel)
	if cmd == nil {
		t.Fatal("q during the confirmation did not quit")
	}
	if quit, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("q during the confirmation did not quit: %T", quit)
	}
	if node.stops != 0 {
		t.Errorf("the abandoned confirmation sent a stop: %d", node.stops)
	}
	if m.confirm {
		t.Fatal("the confirmation is still open")
	}
}

// A slow-group answer from a superseded round is discarded, not painted —
// the generation guard works for the cloud group, not only the local one.
func TestDashModelSlowStaleRoundDiscarded(t *testing.T) {
	node := newFakeDashNode("stopped")
	m := &dashModel{
		entries: []dashEntry{{name: "a", kind: fleet.KindRemote, node: node}},
		results: make([]fleet.NodeResult, 1),
		actions: make([]dashAction, 1),
		width:   120, height: 40,
	}
	m.slowBusy = true
	m.slowGen = 3
	before := m.results[0]
	stale := dashRefreshMsg{
		remote:  true,
		gen:     2, // superseded: the model is on generation 3
		idx:     []int{0},
		results: []fleet.NodeResult{{Name: "a", Outcome: fleet.OutcomeOK, Status: daemon.StatusResponse{State: "running"}}},
	}
	next, _ := m.Update(stale)
	m = next.(*dashModel)
	if m.results[0].Name != before.Name || m.results[0].Outcome != before.Outcome || m.results[0].Err != before.Err {
		t.Errorf("a superseded slow round was painted: %+v", m.results[0])
	}
	if !m.slowBusy {
		t.Error("a stale slow round cleared the in-flight flag")
	}
}

// A slow round already in flight is not started again when its deadline
// comes due: the guard is the busy flag, not the deadline.
func TestDashModelSlowRoundInFlightGuard(t *testing.T) {
	node := newFakeDashNode("running")
	m := &dashModel{
		entries: []dashEntry{{name: "a", kind: fleet.KindRemote, node: node}},
		results: make([]fleet.NodeResult, 1),
		actions: make([]dashAction, 1),
		width:   120, height: 40,
	}
	m.slowBusy = true
	m.nextSlowAt = time.Time{} // the deadline is due
	if cmd := m.refreshRemoteGroup(true); cmd != nil {
		t.Fatal("started a second slow round over one in flight")
	}
}

// A refused start lands on the status line in the one-shot wording, and a
// success that reports no state says done.
func TestDashModelStartOutcomeWordings(t *testing.T) {
	fail := newFakeDashNode("stopped")
	fail.startErr = errors.New("boot exploded")
	empty := newFakeDashNode("stopped")
	empty.startReply = &daemon.StatusResponse{}
	m := &dashModel{
		entries: []dashEntry{
			{name: "a", kind: fleet.KindDaemon, node: fail},
			{name: "b", kind: fleet.KindDaemon, node: empty},
		},
		results: make([]fleet.NodeResult, 2),
		actions: make([]dashAction, 2),
		width:   120, height: 40,
	}
	_, cmd := m.Update(dashKey("s"))
	msgA, _ := cmd().(dashActionMsg)
	next, _ := m.Update(msgA)
	m = next.(*dashModel)
	if m.statusLine != "a: start failed — boot exploded" {
		t.Errorf("failure wording: %q", m.statusLine)
	}
	if m.actions[0].verb != "" {
		t.Errorf("the failed action was not cleared: %+v", m.actions[0])
	}
	if fail.starts != 1 {
		t.Fatal("the failing start was not made")
	}
	// Two columns, two nodes: b sits beside a in the same row.
	next, _ = m.Update(dashKey("right"))
	m = next.(*dashModel)
	_, cmd = m.Update(dashKey("s"))
	msgB, _ := cmd().(dashActionMsg)
	next, _ = m.Update(msgB)
	m = next.(*dashModel)
	if m.statusLine != "b: start — done" {
		t.Errorf("no-state success wording: %q", m.statusLine)
	}
}

// A node without the progress capability starts through the plain verb, and
// says nothing on the way — the tile shows the verb alone.
func TestDashModelStartOnPlainNode(t *testing.T) {
	f := newFakeDashNode("stopped")
	var caught []tea.Msg
	m := &dashModel{
		entries: []dashEntry{{name: "a", kind: fleet.KindDaemon, node: plainDashNode{f}}},
		results: []fleet.NodeResult{{Name: "a"}},
		actions: make([]dashAction, 1),
		send:    func(msg tea.Msg) { caught = append(caught, msg) },
		width:   120, height: 40,
	}
	next, cmd := m.Update(dashKey("s"))
	m = next.(*dashModel)
	if cmd == nil {
		t.Fatal("s did not start the plain node")
	}
	if m.actions[0].verb != "start" {
		t.Fatalf("no action recorded: %+v", m.actions[0])
	}
	if v := m.View(); !strings.Contains(v, "a  starting") {
		t.Errorf("the in-flight tile does not carry the verb:\n%s", v)
	}
	msg, _ := cmd().(dashActionMsg)
	next, _ = m.Update(msg)
	m = next.(*dashModel)
	if f.starts != 1 {
		t.Fatal("the plain start was not made")
	}
	if len(caught) != 0 {
		t.Errorf("a plain start reported progress: %v", caught)
	}
	if m.statusLine != "a: start — running" {
		t.Errorf("status line: %q", m.statusLine)
	}
}

// beginAction itself refuses a node that already carries an action, in its
// own words — the keys gate first, so this is the guard behind them.
func TestDashModelBeginActionRefusesANodeStillWorking(t *testing.T) {
	node := newFakeDashNode("running")
	m := &dashModel{
		entries: []dashEntry{{name: "a", kind: fleet.KindDaemon, node: node}},
		results: []fleet.NodeResult{{Name: "a"}},
		actions: make([]dashAction, 1),
		width:   120, height: 40,
	}
	m.actions[0] = dashAction{verb: "start"}
	if cmd := m.beginAction("start"); cmd != nil {
		t.Fatal("started a node still starting")
	}
	if m.statusLine != "a: still starting" {
		t.Errorf("status line: %q", m.statusLine)
	}
}

// The abort ends the wait on an in-flight start: the call's own loop comes
// back on the done context, the tile clears, the line says the wait was
// abandoned — not the node failed — and the node is free to start again.
func TestDashModelAbortsAnInFlightStart(t *testing.T) {
	hold := make(chan struct{})
	f := newFakeDashNode("stopped")
	f.hold = hold // the start stays in flight until released or cancelled
	m := &dashModel{
		entries: []dashEntry{{name: "a", kind: fleet.KindRemote, node: f}},
		results: []fleet.NodeResult{{Name: "a"}},
		actions: make([]dashAction, 1),
		width:   120, height: 40,
	}
	next, cmd := m.Update(dashKey("s"))
	m = next.(*dashModel)
	if m.actions[0].verb != "start" {
		t.Fatalf("no action recorded: %+v", m.actions[0])
	}
	// A second start is gated by the key itself: nothing is driven while the
	// first is in flight.
	next, cmdRefused := m.Update(dashKey("s"))
	m = next.(*dashModel)
	if cmdRefused != nil {
		t.Fatalf("a second start was sent: %v", cmdRefused)
	}
	// The abort marks the action and cancels the call's context.
	next, _ = m.Update(dashKey("a"))
	m = next.(*dashModel)
	if !m.actions[0].aborted {
		t.Fatal("the abort did not mark the action")
	}
	// The call's loop returns on the done context, and its final message lands
	// as for any finished action.
	msg, _ := cmd().(dashActionMsg)
	if msg.err == nil {
		t.Fatalf("the aborted start came back as a success: %+v", msg)
	}
	next, _ = m.Update(msg)
	m = next.(*dashModel)
	if m.actions[0].verb != "" || m.actions[0].aborted {
		t.Fatalf("the action was not cleared: %+v", m.actions[0])
	}
	if m.statusLine != "a: start abandoned" {
		t.Errorf("status line: %q", m.statusLine)
	}
	if v := m.View(); strings.Contains(v, "starting") {
		t.Errorf("the tile still carries the aborted start:\n%s", v)
	}
	// The freed node takes a second start, and it succeeds on release.
	next, cmd2 := m.Update(dashKey("s"))
	m = next.(*dashModel)
	if m.actions[0].verb != "start" {
		t.Fatalf("the freed node did not take a second start: %+v", m.actions[0])
	}
	close(hold)
	msg2, _ := cmd2().(dashActionMsg)
	next, _ = m.Update(msg2)
	m = next.(*dashModel)
	if m.statusLine != "a: start — running" {
		t.Errorf("second start: %q", m.statusLine)
	}
	// The aborted start never reached the call (it was still on its hold
	// line); only the second did.
	if f.starts != 1 {
		t.Errorf("starts = %d (want 1)", f.starts)
	}
}

// a on a node with nothing in flight drives nothing: no action, no line, no
// command.
func TestDashModelAbortOnAnIdleNodeDrivesNothing(t *testing.T) {
	node := newFakeDashNode("running")
	m := &dashModel{
		entries: []dashEntry{{name: "a", kind: fleet.KindDaemon, node: node}},
		results: []fleet.NodeResult{{Name: "a"}},
		actions: make([]dashAction, 1),
		width:   120, height: 40,
	}
	m.statusLine = "an earlier outcome"
	next, cmd := m.Update(dashKey("a"))
	m = next.(*dashModel)
	if cmd != nil {
		t.Errorf("the idle abort sent a command: %v", cmd)
	}
	if m.actions[0].verb != "" || m.actions[0].aborted {
		t.Errorf("the idle abort touched the action: %+v", m.actions[0])
	}
	if m.statusLine != "an earlier outcome" {
		t.Errorf("status line: %q", m.statusLine)
	}
}

// A success that lands with the abort is reported as the success — the node
// is running, and "abandoned" over a green node would be a lie — so the
// finished message wins when it carries no error.
func TestDashModelRacingSuccessIsReportedAsSuccess(t *testing.T) {
	node := newFakeDashNode("stopped")
	m := &dashModel{
		entries: []dashEntry{{name: "a", kind: fleet.KindRemote, node: node}},
		results: []fleet.NodeResult{{Name: "a"}},
		actions: make([]dashAction, 1),
		width:   120, height: 40,
	}
	m.actions[0] = dashAction{verb: "start", aborted: true}
	next, _ := m.Update(dashActionMsg{node: "a", verb: "start", status: daemon.StatusResponse{State: "running"}})
	m = next.(*dashModel)
	if m.statusLine != "a: start — running" {
		t.Errorf("status line: %q", m.statusLine)
	}
	if m.actions[0].verb != "" {
		t.Errorf("the action was not cleared: %+v", m.actions[0])
	}
}

// The line's own decision, at its level: an aborted failure is the
// abandonment, an un-aborted one the failure, a success the success whatever
// the abort says.
func TestDashActionLineAbortedWording(t *testing.T) {
	cases := []struct {
		aborted bool
		err     error
		state   string
		want    string
	}{
		{false, errors.New("boot exploded"), "", "a: start failed — boot exploded"},
		{true, errors.New("context canceled"), "", "a: start abandoned"},
		{true, nil, "running", "a: start — running"},
		{false, nil, "", "a: start — done"},
	}
	for i, c := range cases {
		if got := dashActionLine(dashActionMsg{node: "a", verb: "start", err: c.err, status: daemon.StatusResponse{State: c.state}}, c.aborted); got != c.want {
			t.Errorf("case %d: %q (want %q)", i, got, c.want)
		}
	}
}

// A line for a node the board does not know is dropped, and a final for one
// still leaves its line on the footer without touching a real node.
func TestDashModelUnknownNodeMessagesIgnored(t *testing.T) {
	node := newFakeDashNode("running")
	m := &dashModel{
		entries: []dashEntry{{name: "a", kind: fleet.KindDaemon, node: node}},
		results: []fleet.NodeResult{{Name: "a"}},
		actions: make([]dashAction, 1),
		width:   120, height: 40,
	}
	next, _ := m.Update(dashActionProgressMsg{node: "ghost", line: "a line"})
	m = next.(*dashModel)
	if m.actions[0].line != "" {
		t.Errorf("a stranger's line landed on the wrong tile: %+v", m.actions[0])
	}
	next, _ = m.Update(dashActionMsg{node: "ghost", verb: "start", status: daemon.StatusResponse{State: "running"}})
	m = next.(*dashModel)
	if m.actions[0].verb != "" {
		t.Errorf("a stranger's final cleared this node: %+v", m.actions[0])
	}
	if m.statusLine != "ghost: start — running" {
		t.Errorf("footer: %q", m.statusLine)
	}
}

// A board with no entries answers its keys and draws without panicking.
func TestDashModelEmptyFleet(t *testing.T) {
	m := &dashModel{width: 120, height: 40}
	m.keepVisible()
	if m.scrollRow != 0 {
		t.Fatalf("keepVisible on nothing: %d", m.scrollRow)
	}
	for _, key := range []string{"up", "down", "left", "right", "r", "s", "x", "a", "enter", "pgup", "pgdown"} {
		next, _ := m.Update(dashKey(key))
		m = next.(*dashModel)
	}
	if m.confirm {
		t.Error("x opened a confirmation on a board with no nodes")
	}
	if m.detail {
		t.Error("enter opened the detail view on a board with no nodes")
	}
	if v := m.View(); v == "" {
		t.Error("an empty board drew nothing")
	}
}

// The end-to-end path: the real program against in-memory nodes — a tile
// turns on with s, the stop confirmation names the node, and y turns it
// off. q leaves with the terminal restored, so WaitFinished sees a clean
// exit.
func TestDashProgramStartsAndStopsANode(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	interval := dashboardRefreshInterval
	dashboardRefreshInterval = 15 * time.Millisecond
	defer func() { dashboardRefreshInterval = interval }()

	node := newFakeDashNode("stopped")
	release := make(chan struct{})
	node.hold = release // hold the start on its first status line until told
	// The progress door, as runDashProgram wires it — through an atomic box,
	// because the program is already rendering (and a value-receiver View
	// copies the whole model, send word included) by the time the test can
	// hand it the program's Send. A start that flies before the wiring
	// reports into nothing, the model's nil path.
	var door atomic.Pointer[func(tea.Msg)]
	m := &dashModel{
		fleetPath: "fleet.yaml",
		entries:   []dashEntry{{name: "alpha", kind: fleet.KindDaemon, node: node}},
		results:   []fleet.NodeResult{{Name: "alpha"}},
		actions:   make([]dashAction, 1),
		send: func(msg tea.Msg) {
			if p := door.Load(); p != nil {
				(*p)(msg)
			}
		},
	}
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))
	send := tm.GetProgram().Send
	door.Store(&send)
	out := tm.Output()
	seen := func(what string, d time.Duration) {
		teatest.WaitFor(t, out, func(b []byte) bool { return bytes.Contains(b, []byte(what)) }, teatest.WithDuration(d))
	}
	seen("stopped", 5*time.Second)
	tm.Type("s")
	// The call reports itself mid-flight — through the program's Send, onto
	// the tile — and stays there until it finishes.
	seen("instance starting; retrying in 1s", 5*time.Second)
	close(release)
	seen("alpha  running", 5*time.Second)
	tm.Type("x")
	seen("stop alpha?", 5*time.Second)
	tm.Type("y")
	seen("stop — stopped", 5*time.Second)
	tm.Type("q")
	tm.WaitFinished(t)
	if node.starts != 1 || node.stops != 1 {
		t.Fatalf("starts=%d stops=%d, want 1 1", node.starts, node.stops)
	}
}

// The piped invocation fails before any part of the view exists, and names
// the command that carries the same data into a pipe.
func TestFleetDashboardRefusesPipedOutput(t *testing.T) {
	captureStdout(t, func() {
		err := cmdFleet([]string{"dashboard"})
		if err == nil {
			t.Fatal("dashboard ran without a terminal")
		}
		if !strings.Contains(err.Error(), "fleet metrics --watch") {
			t.Fatalf("error does not name the piped alternative: %v", err)
		}
	})
}

// A fleet file that cannot be read fails the command before the view, and a
// node whose token reference is unresolvable is a standing row, not a gap.
func TestDashModelForFleetFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if _, err := dashModelFor(""); err == nil {
		t.Fatal("missing fleet file did not fail")
	}
	writeFleetFile(t, "nodes:\n  - name: ok\n    host: 127.0.0.1\n    port: 4242\n  - name: broken\n    host: 127.0.0.1\n    port: 1\n    tokenEnv: NO_SUCH_SPINLOOP_VAR\n")
	m, err := dashModelFor("")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.entries) != 2 || m.entries[0].node == nil {
		t.Fatalf("a healthy entry did not become a node: %+v", m.entries)
	}
	if m.entries[1].node != nil {
		t.Fatalf("the broken entry became a node: %+v", m.entries[1])
	}
	if m.entries[1].standing.Outcome != fleet.OutcomeConfigError {
		t.Fatalf("standing outcome: %v", m.entries[1].standing.Outcome)
	}
	// A frame tall enough to show both rows: the model carries no size until
	// the window reports one, and the default is short enough to scroll.
	m.width, m.height = 120, 40
	view := m.View()
	if !strings.Contains(view, "broken") || !strings.Contains(view, "config-error") {
		t.Errorf("board does not show the broken node:\n%s", view)
	}
}

// startDetailRound begins the detail view's log round through the model's
// own door, bypassing the ticking wrapper, and runs it to completion — the
// detail-view analogue of startFastRound.
func startDetailRound(t *testing.T, m *dashModel) dashDetailLogMsg {
	t.Helper()
	cmd := m.startDetailLogRound()
	if cmd == nil {
		t.Fatal("no log round to run")
	}
	msg, ok := cmd().(dashDetailLogMsg)
	if !ok {
		t.Fatalf("log round cmd answered %T, want dashDetailLogMsg", msg)
	}
	return msg
}

// Enter opens the detail view on the node under the cursor without moving
// the selection; escape closes it and returns to the grid with the same
// selection.
func TestDashModelEnterOpensDetailEscapeReturns(t *testing.T) {
	m := &dashModel{
		entries: []dashEntry{
			{name: "a", kind: fleet.KindDaemon, node: newFakeDashNode("stopped")},
			{name: "b", kind: fleet.KindDaemon, node: newFakeDashNode("stopped")},
		},
		results: []fleet.NodeResult{{Name: "a"}, {Name: "b"}},
		actions: make([]dashAction, 2),
		cursor:  1,
		width:   80, height: 24,
	}
	m2, _ := m.Update(dashKey("enter"))
	mm := m2.(*dashModel)
	if !mm.detail {
		t.Fatal("enter did not open the detail view")
	}
	if mm.cursor != 1 {
		t.Fatalf("enter moved the selection: %d", mm.cursor)
	}
	if !strings.Contains(mm.View(), "node: b") {
		t.Errorf("detail view is not showing the selected node:\n%s", mm.View())
	}
	m3, _ := mm.Update(dashKey("esc"))
	mm2 := m3.(*dashModel)
	if mm2.detail {
		t.Fatal("escape did not close the detail view")
	}
	if mm2.cursor != 1 {
		t.Fatalf("escape moved the selection: %d", mm2.cursor)
	}
	if !strings.Contains(mm2.View(), "↑↓←→ move") {
		t.Errorf("escape did not return to the grid:\n%s", mm2.View())
	}
}

// The detail view drives the node it shows through the same start/stop/abort
// rules the grid applies to the node under the cursor — which is the same
// node, since the cursor never moves while the view is open.
func TestDashModelDetailKeysDriveTheNodeInView(t *testing.T) {
	node := newFakeDashNode("stopped")
	m := &dashModel{
		entries: []dashEntry{{name: "a", kind: fleet.KindDaemon, node: node}},
		results: []fleet.NodeResult{{Name: "a"}},
		actions: make([]dashAction, 1),
		width:   80, height: 24,
	}
	m2, _ := m.Update(dashKey("enter"))
	mm := m2.(*dashModel)

	m3, cmd := mm.Update(dashKey("s"))
	mm = m3.(*dashModel)
	if cmd == nil || mm.actions[0].verb != "start" {
		t.Fatalf("s from the detail view did not start the node: %+v", mm.actions[0])
	}
	smsg, _ := cmd().(dashActionMsg)
	m4, _ := mm.Update(smsg)
	mm = m4.(*dashModel)
	if !strings.Contains(mm.statusLine, "a: start — running") {
		t.Errorf("start status line: %q", mm.statusLine)
	}
	if !mm.detail {
		t.Fatal("the detail view closed on its own after a start")
	}

	m5, _ := mm.Update(dashKey("x"))
	mm = m5.(*dashModel)
	if !mm.confirm {
		t.Fatal("x from the detail view did not ask for confirmation")
	}
	m6, cmd2 := mm.Update(dashKey("y"))
	mm = m6.(*dashModel)
	if cmd2 == nil {
		t.Fatal("y did not stop from the detail view")
	}
	pmsg, _ := cmd2().(dashActionMsg)
	m7, _ := mm.Update(pmsg)
	mm = m7.(*dashModel)
	if !strings.Contains(mm.statusLine, "a: stop — stopped") {
		t.Errorf("stop status line: %q", mm.statusLine)
	}
	if !mm.detail {
		t.Fatal("the detail view closed on its own after a stop")
	}
}

// The abort key ends the wait on the node's in-flight action from inside the
// detail view, the same as it does from the grid.
func TestDashModelDetailAbort(t *testing.T) {
	m := &dashModel{
		entries: []dashEntry{{name: "a", kind: fleet.KindDaemon, node: newFakeDashNode("stopped")}},
		results: []fleet.NodeResult{{Name: "a"}},
		actions: make([]dashAction, 1),
		detail:  true,
		width:   80, height: 24,
	}
	aborted := false
	m.actions[0] = dashAction{verb: "start", cancel: func() { aborted = true }}
	m2, _ := m.Update(dashKey("a"))
	mm := m2.(*dashModel)
	if !aborted {
		t.Fatal("a from the detail view did not abort the in-flight action")
	}
	if !mm.actions[0].aborted {
		t.Fatal("the action was not marked aborted")
	}
}

// Abort only ends the wait on a start — the one action with no deadline of
// its own. A stop in flight targets an engine already running and is not
// abortable: abandoning the wait would leave the operator unsure whether the
// stop still went ahead, so the abort key from either surface drives nothing
// on it.
func TestDashModelAbortRefusesOnAStopInFlight(t *testing.T) {
	m := &dashModel{
		entries: []dashEntry{{name: "a", kind: fleet.KindDaemon, node: newFakeDashNode("running")}},
		results: []fleet.NodeResult{{Name: "a"}},
		actions: make([]dashAction, 1),
		width:   80, height: 24,
	}
	cancelled := false
	m.actions[0] = dashAction{verb: "stop", cancel: func() { cancelled = true }}
	m.abortAction()
	if cancelled {
		t.Fatal("abort cancelled a stop in flight")
	}
	if m.actions[0].aborted {
		t.Fatal("a stop in flight was marked aborted")
	}
	if m.actions[0].verb != "stop" {
		t.Fatalf("the stop's action state changed: %+v", m.actions[0])
	}
}

// The refusal holds through the grid's own key.
func TestDashModelGridAbortKeyRefusesOnAStopInFlight(t *testing.T) {
	m := &dashModel{
		entries: []dashEntry{{name: "a", kind: fleet.KindDaemon, node: newFakeDashNode("running")}},
		results: []fleet.NodeResult{{Name: "a"}},
		actions: []dashAction{{verb: "stop"}},
		width:   80, height: 24,
	}
	m2, _ := m.Update(dashKey("a"))
	mm := m2.(*dashModel)
	if mm.actions[0].aborted {
		t.Fatal("a on the grid aborted a stop in flight")
	}
}

// And through the detail view's own key.
func TestDashModelDetailAbortRefusesOnAStopInFlight(t *testing.T) {
	m := &dashModel{
		entries: []dashEntry{{name: "a", kind: fleet.KindDaemon, node: newFakeDashNode("running")}},
		results: []fleet.NodeResult{{Name: "a"}},
		actions: []dashAction{{verb: "stop"}},
		detail:  true,
		width:   80, height: 24,
	}
	m2, _ := m.Update(dashKey("a"))
	mm := m2.(*dashModel)
	if mm.actions[0].aborted {
		t.Fatal("a from the detail view aborted a stop in flight")
	}
}

func TestDashCanAbort(t *testing.T) {
	cases := []struct {
		name string
		verb string
		want bool
	}{
		{"idle, nothing in flight", "", false},
		{"a start in flight", "start", true},
		{"a stop in flight", "stop", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := dashModel{
				entries: []dashEntry{{name: "a", kind: fleet.KindDaemon}},
				actions: []dashAction{{verb: tc.verb}},
			}
			if got := m.canAbort(); got != tc.want {
				t.Errorf("canAbort() = %v, want %v", got, tc.want)
			}
		})
	}
	if (dashModel{}).canAbort() {
		t.Error("canAbort() on an empty fleet")
	}
}

func TestDashFooterHints(t *testing.T) {
	const hints = "j/k move   s start   a abort   x stop   r refresh   q quit"
	if got := dashFooterHints(hints, true); got != hints {
		t.Errorf("abortable dropped or changed hints: %q", got)
	}
	want := "j/k move   s start   x stop   r refresh   q quit"
	if got := dashFooterHints(hints, false); got != want {
		t.Errorf("dashFooterHints(false) = %q, want %q", got, want)
	}
}

// The footer only advertises abort while a start is actually in flight on
// the node it describes — not for an idle or running node, and not for one
// whose in-flight action is a stop, both of which would make the key a
// no-op if pressed.
func TestDashGridFooterOmitsAbortWhenNothingIsAbortable(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	m := dashModel{
		entries: []dashEntry{{name: "a", kind: fleet.KindDaemon}},
		results: []fleet.NodeResult{{Name: "a", Outcome: fleet.OutcomeOK, Metrics: metrics.Stats{State: "running"}}},
		actions: make([]dashAction, 1),
		width:   80, height: 24,
	}
	if v := m.View(); strings.Contains(v, "a abort") {
		t.Errorf("grid footer offers abort for a running node with nothing in flight:\n%s", v)
	}
	m.actions[0] = dashAction{verb: "stop"}
	if v := m.View(); strings.Contains(v, "a abort") {
		t.Errorf("grid footer offers abort while a stop is in flight:\n%s", v)
	}
	m.actions[0] = dashAction{verb: "start"}
	if v := m.View(); !strings.Contains(v, "a abort") {
		t.Errorf("grid footer hides abort while a start is in flight:\n%s", v)
	}
}

// Same rule from the detail view.
func TestDashDetailFooterOmitsAbortWhenNothingIsAbortable(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	m := dashModel{
		entries: []dashEntry{{name: "a", kind: fleet.KindDaemon}},
		results: []fleet.NodeResult{{Name: "a", Outcome: fleet.OutcomeOK, Metrics: metrics.Stats{State: "running"}}},
		actions: make([]dashAction, 1),
		detail:  true,
		width:   80, height: 24,
	}
	if v := m.detailView(); strings.Contains(v, "a abort") {
		t.Errorf("detail footer offers abort for a running node with nothing in flight:\n%s", v)
	}
	m.actions[0] = dashAction{verb: "stop"}
	if v := m.detailView(); strings.Contains(v, "a abort") {
		t.Errorf("detail footer offers abort while a stop is in flight:\n%s", v)
	}
	m.actions[0] = dashAction{verb: "start"}
	if v := m.detailView(); !strings.Contains(v, "a abort") {
		t.Errorf("detail footer hides abort while a start is in flight:\n%s", v)
	}
}

// Quit lives on the grid only: q and ctrl+c inside the detail view drive
// nothing, and the view stays open — the operator escapes back first.
func TestDashModelDetailQuitIsGridOnly(t *testing.T) {
	m := &dashModel{
		entries: []dashEntry{{name: "a", kind: fleet.KindDaemon, node: newFakeDashNode("stopped")}},
		results: []fleet.NodeResult{{Name: "a"}},
		actions: make([]dashAction, 1),
		detail:  true,
		width:   80, height: 24,
	}
	for _, key := range []string{"q", "ctrl+c"} {
		next, cmd := m.Update(dashKey(key))
		mm := next.(*dashModel)
		if cmd != nil {
			t.Fatalf("%s from the detail view returned a command: %T", key, cmd())
		}
		if !mm.detail {
			t.Fatalf("%s closed the detail view", key)
		}
	}
}

// Opening the detail view on an entry that never became a node (a standing
// config-error) shows the standing outcome as the log pane's explanation
// rather than starting a log round that could never answer.
func TestDashModelOpenDetailOnStandingNode(t *testing.T) {
	standing := fleet.NodeResult{Name: "broken", Outcome: fleet.OutcomeConfigError, Err: errors.New(`tokenEnv "NOPE" is set nowhere`)}
	m := &dashModel{
		entries: []dashEntry{{name: "broken", kind: fleet.KindDaemon, standing: standing}},
		results: []fleet.NodeResult{standing},
		actions: make([]dashAction, 1),
		width:   80, height: 24,
	}
	if cmd := m.openDetail(); cmd != nil {
		t.Fatal("a node with no live node started a log round")
	}
	if !m.detail {
		t.Fatal("the detail view did not open on a standing node")
	}
	if !strings.Contains(m.detailLogNote, "set nowhere") {
		t.Errorf("the standing outcome was not shown as the log note: %q", m.detailLogNote)
	}
	view := m.detailView()
	if !strings.Contains(view, "config-error") || !strings.Contains(view, "set nowhere") {
		t.Errorf("detail view does not show the standing outcome:\n%s", view)
	}
}

// New content lands on every poll and accumulates across them.
func TestDashDetailLogAccumulatesAcrossPolls(t *testing.T) {
	node := newFakeDashNode("running")
	node.appendLog("first line\n")
	m := &dashModel{
		entries: []dashEntry{{name: "a", kind: fleet.KindDaemon, node: node}},
		results: []fleet.NodeResult{{Name: "a"}},
		actions: make([]dashAction, 1),
		width:   80, height: 24,
	}
	m.openDetail()
	m.detailLogBusy = false // drive the round below directly, not the one opening already started
	next, _ := m.Update(startDetailRound(t, m))
	m = next.(*dashModel)
	if !strings.Contains(m.detailLogContent, "first line") {
		t.Fatalf("log content missing after the first poll: %q", m.detailLogContent)
	}

	node.appendLog("second line\n")
	next2, _ := m.Update(startDetailRound(t, m))
	m = next2.(*dashModel)
	if !strings.Contains(m.detailLogContent, "first line") || !strings.Contains(m.detailLogContent, "second line") {
		t.Fatalf("log content did not accumulate: %q", m.detailLogContent)
	}
}

// The accumulated log is trimmed to what the pane can show on every poll, so
// a long session never grows an unbounded buffer.
func TestDashDetailLogTrimmedToPaneCapacity(t *testing.T) {
	node := newFakeDashNode("stopped")
	node.appendLog("l1\nl2\nl3\nl4\nl5\n")
	m := &dashModel{
		entries: []dashEntry{{name: "a", kind: fleet.KindDaemon, node: node}},
		results: []fleet.NodeResult{{Name: "a"}}, // unanswered: a 2-line metrics section
		actions: make([]dashAction, 1),
		width:   80, height: 10,
	}
	m.openDetail()
	m.detailLogBusy = false // drive the round below directly, not the one opening already started
	next, _ := m.Update(startDetailRound(t, m))
	m = next.(*dashModel)
	if got, want := m.detailLogContent, "l3\nl4\nl5\n"; got != want {
		t.Fatalf("trimmed log content = %q, want %q", got, want)
	}
}

// A reply that lands after the view has closed, or after it has reopened
// (its own generation moved on), is discarded rather than painted.
func TestDashDetailLogStaleReplyDiscarded(t *testing.T) {
	node := newFakeDashNode("running")
	m := &dashModel{
		entries: []dashEntry{{name: "a", kind: fleet.KindDaemon, node: node}},
		results: []fleet.NodeResult{{Name: "a"}},
		actions: make([]dashAction, 1),
		width:   80, height: 24,
	}
	m.openDetail()
	m.detailLogBusy = false // drive the round below directly, not the one opening already started
	node.appendLog("late reply\n")
	msg := startDetailRound(t, m)

	// Closed before the round lands.
	m.detail = false
	next, _ := m.Update(msg)
	m = next.(*dashModel)
	if m.detailLogContent != "" {
		t.Fatalf("a reply after close was applied: %q", m.detailLogContent)
	}

	// Reopened since — a fresh generation — before the same reply lands.
	m.openDetail()
	next2, _ := m.Update(msg)
	m = next2.(*dashModel)
	if m.detailLogContent != "" {
		t.Fatalf("a stale-generation reply was applied: %q", m.detailLogContent)
	}
}

// The log poll does not outlive the view: the tick's own handler declines to
// reschedule once the view has closed, which is the one shutdown path.
func TestDashDetailLogTickStopsOnClose(t *testing.T) {
	m := &dashModel{
		entries: []dashEntry{{name: "a", kind: fleet.KindDaemon, node: newFakeDashNode("running")}},
		results: []fleet.NodeResult{{Name: "a"}},
		actions: make([]dashAction, 1),
		width:   80, height: 24,
	}
	if _, cmd := m.Update(detailLogTickMsg{}); cmd != nil {
		t.Fatal("the tick rescheduled itself while the view was never open")
	}
	m.detail = true
	if _, cmd := m.Update(detailLogTickMsg{}); cmd == nil {
		t.Fatal("the tick did not reschedule while the view was open")
	}
	m.detail = false
	if _, cmd := m.Update(detailLogTickMsg{}); cmd != nil {
		t.Fatal("the tick rescheduled itself after the view closed")
	}
}

// A tick scheduled while viewing a live node must not resurrect a poll loop
// if it lands after the operator has switched to (or reopened on) an entry
// f pauses the log poll and resumes it: while paused the tick chain keeps
// ticking (so a later resume needs nothing but the flag) but starts no
// round; f again picks the round straight back up on the next tick.
func TestDashModelDetailFollowToggle(t *testing.T) {
	m := &dashModel{
		entries: []dashEntry{{name: "a", kind: fleet.KindDaemon, node: newFakeDashNode("running")}},
		results: []fleet.NodeResult{{Name: "a"}},
		actions: make([]dashAction, 1),
		width:   80, height: 24,
	}
	m2, _ := m.Update(dashKey("enter"))
	mm := m2.(*dashModel)
	if !mm.detailLogFollow {
		t.Fatal("the log does not follow by default when the view opens")
	}
	mm.detailLogBusy = false // opening already started its own round; not what this test is about

	m3, _ := mm.Update(dashKey("f"))
	mm = m3.(*dashModel)
	if mm.detailLogFollow {
		t.Fatal("f did not pause the follow")
	}

	m4, cmd := mm.Update(detailLogTickMsg{})
	mm = m4.(*dashModel)
	if cmd == nil {
		t.Fatal("the tick chain died while paused instead of just skipping the round")
	}
	if mm.detailLogBusy {
		t.Fatal("a round started while paused")
	}

	m5, _ := mm.Update(dashKey("f"))
	mm = m5.(*dashModel)
	if !mm.detailLogFollow {
		t.Fatal("f did not resume the follow")
	}
	if _, cmd2 := mm.Update(detailLogTickMsg{}); cmd2 == nil {
		t.Fatal("no round started on the tick after resuming")
	}
}

// The header names the log's follow state for a node that can actually poll
// one, and says nothing about a state a standing node can never have.
func TestDashDetailViewHeaderShowsFollowState(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	m := dashModel{
		entries:         []dashEntry{{name: "n", kind: fleet.KindDaemon, node: newFakeDashNode("stopped")}},
		results:         []fleet.NodeResult{{Name: "n"}},
		actions:         make([]dashAction, 1),
		detail:          true,
		detailLogFollow: true,
		width:           80, height: 24,
	}
	if v := m.detailView(); !strings.Contains(v, "log: following") {
		t.Errorf("header does not show following:\n%s", v)
	}
	m.detailLogFollow = false
	if v := m.detailView(); !strings.Contains(v, "log: paused") {
		t.Errorf("header does not show paused:\n%s", v)
	}
}

func TestDashDetailViewHeaderOmitsFollowStateForStandingNode(t *testing.T) {
	m := dashModel{
		entries: []dashEntry{{name: "broken", kind: fleet.KindDaemon}},
		results: []fleet.NodeResult{{Name: "broken", Outcome: fleet.OutcomeConfigError}},
		actions: make([]dashAction, 1),
		detail:  true,
		width:   80, height: 24,
	}
	if v := m.detailView(); strings.Contains(v, "log:") {
		t.Errorf("a standing node's header claims a log state it can never have:\n%s", v)
	}
}

// End to end: f actually stops new log content from appearing, and f again
// lets it through.
func TestDashProgramDetailFollowToggle(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	logInterval := detailLogInterval
	detailLogInterval = 15 * time.Millisecond
	defer func() { detailLogInterval = logInterval }()

	node := newFakeDashNode("stopped")
	m := &dashModel{
		fleetPath: "fleet.yaml",
		entries:   []dashEntry{{name: "alpha", kind: fleet.KindDaemon, node: node}},
		results:   []fleet.NodeResult{{Name: "alpha"}},
		actions:   make([]dashAction, 1),
	}
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))
	out := tm.Output()
	seen := func(what string, d time.Duration) {
		teatest.WaitFor(t, out, func(b []byte) bool { return bytes.Contains(b, []byte(what)) }, teatest.WithDuration(d))
	}
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	seen("log: following", 5*time.Second)
	tm.Type("f")
	seen("log: paused", 5*time.Second)

	node.appendLog("should not appear yet\n")
	time.Sleep(100 * time.Millisecond) // several paused-interval ticks' worth
	if drained, err := io.ReadAll(out); err != nil {
		t.Fatal(err)
	} else if bytes.Contains(drained, []byte("should not appear yet")) {
		t.Fatal("the log kept polling while paused")
	}

	tm.Type("f")
	seen("should not appear yet", 5*time.Second)
	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
	tm.Type("q")
	tm.WaitFinished(t)
}

// with no live node — that entry's view never schedules a tick of its own,
// and a stray one from before must not start doing so on its behalf.
func TestDashDetailLogTickDoesNotResurrectOnANodelessView(t *testing.T) {
	m := &dashModel{
		entries: []dashEntry{{name: "broken", kind: fleet.KindDaemon}},
		results: []fleet.NodeResult{{Name: "broken"}},
		actions: make([]dashAction, 1),
		detail:  true,
	}
	if _, cmd := m.Update(detailLogTickMsg{}); cmd != nil {
		t.Fatal("a stale tick rescheduled itself over a node that can never answer")
	}
}

// startDetailLogRound refuses on its own — independent of any caller's own
// guard — both for an entry with no live node and for one already in flight.
func TestDashDetailLogRoundGuards(t *testing.T) {
	m := &dashModel{
		entries: []dashEntry{{name: "broken", kind: fleet.KindDaemon}},
		results: []fleet.NodeResult{{Name: "broken"}},
		actions: make([]dashAction, 1),
	}
	if cmd := m.startDetailLogRound(); cmd != nil {
		t.Fatal("a round started for an entry with no live node")
	}
	m.entries[0].node = newFakeDashNode("stopped")
	if cmd := m.startDetailLogRound(); cmd == nil {
		t.Fatal("no round started for a live node")
	}
	if cmd := m.startDetailLogRound(); cmd != nil {
		t.Fatal("a second round started while one was already in flight")
	}
}

// A failed poll leaves prior content in place rather than blanking it, and
// only supplies the note while there is nothing else to show.
func TestDashDetailLogFailedPollKeepsPriorContent(t *testing.T) {
	m := &dashModel{
		entries:          []dashEntry{{name: "a", kind: fleet.KindDaemon}},
		results:          []fleet.NodeResult{{Name: "a"}},
		actions:          make([]dashAction, 1),
		detail:           true,
		detailLogContent: "already here\n",
	}
	m.applyDetailLog(fleet.NodeResult{Name: "a", Outcome: fleet.OutcomeUnreachable, Err: errors.New("connection refused")})
	if m.detailLogContent != "already here\n" {
		t.Fatalf("a failed poll discarded prior content: %q", m.detailLogContent)
	}
	if m.detailLogNote != "" {
		t.Fatalf("a failed poll overwrote content with a note: %q", m.detailLogNote)
	}
}

// A failed poll before any content has ever landed shows the failure as the
// pane's explanation.
func TestDashDetailLogFailedPollBeforeAnyContentShowsNote(t *testing.T) {
	m := &dashModel{
		entries: []dashEntry{{name: "a", kind: fleet.KindDaemon}},
		results: []fleet.NodeResult{{Name: "a"}},
		actions: make([]dashAction, 1),
		detail:  true,
	}
	m.applyDetailLog(fleet.NodeResult{Name: "a", Outcome: fleet.OutcomeUnreachable, Err: errors.New("connection refused")})
	if !strings.Contains(m.detailLogNote, "connection refused") {
		t.Errorf("a failed first poll did not explain itself: %q", m.detailLogNote)
	}
}

// A successful poll with nothing new to add leaves prior content and its
// cleared note alone.
func TestDashDetailLogNoNewContentKeepsPriorState(t *testing.T) {
	m := &dashModel{
		entries:          []dashEntry{{name: "a", kind: fleet.KindDaemon}},
		results:          []fleet.NodeResult{{Name: "a"}},
		actions:          make([]dashAction, 1),
		detail:           true,
		detailLogContent: "already here\n",
		detailLogOffset:  5,
	}
	m.applyDetailLog(fleet.NodeResult{Name: "a", Outcome: fleet.OutcomeOK, Logs: daemon.LogsResponse{NextOffset: 5}})
	if m.detailLogContent != "already here\n" {
		t.Fatalf("content changed on an empty successful poll: %q", m.detailLogContent)
	}
	if m.detailLogNote != "" {
		t.Fatalf("a note appeared despite existing content: %q", m.detailLogNote)
	}
	if m.detailLogOffset != 5 {
		t.Fatalf("the offset did not advance to NextOffset: %d", m.detailLogOffset)
	}
}

// The log section is floored at one row even when the terminal is too short
// for the header, footer, dividers and metrics section to leave any room.
func TestDashDetailSectionHeightsFloorsAtOneRow(t *testing.T) {
	m := &dashModel{
		entries: []dashEntry{{name: "a", kind: fleet.KindDaemon}},
		results: []fleet.NodeResult{{Name: "a"}},
		actions: make([]dashAction, 1),
		detail:  true,
		width:   80, height: 1,
	}
	if _, log := m.detailSectionHeights(); log != 1 {
		t.Fatalf("log section height = %d, want the 1-row floor", log)
	}
}

// detailView bounds the log section to the rows actually available even when
// the accumulated content is already longer than that — the same bound
// applyDetailLog enforces on the way in, defended again on the way out.
func TestDashDetailViewTruncatesOverflowingLogToAvailableRows(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	m := dashModel{
		entries:          []dashEntry{{name: "n", kind: fleet.KindDaemon}},
		results:          []fleet.NodeResult{{Name: "n"}}, // a 2-line metrics section
		actions:          make([]dashAction, 1),
		detail:           true,
		detailLogContent: "l1\nl2\nl3\nl4\nl5\n",
		width:            80, height: 10, // avail = 10 - 5 - 2 = 3
	}
	view := m.detailView()
	if strings.Contains(view, "l1") || strings.Contains(view, "l2") {
		t.Errorf("view shows log lines older than the pane can hold:\n%s", view)
	}
	if !strings.Contains(view, "l3") || !strings.Contains(view, "l4") || !strings.Contains(view, "l5") {
		t.Errorf("view is missing the most recent log lines:\n%s", view)
	}
}

func TestDetailLogLinesPlaceholderAndNote(t *testing.T) {
	if got := detailLogLines("", ""); len(got) != 1 || got[0] != "waiting for the log…" {
		t.Errorf("placeholder: %v", got)
	}
	if got := detailLogLines("", "no output"); len(got) != 1 || got[0] != "no output" {
		t.Errorf("note: %v", got)
	}
	if got := detailLogLines("a\nb\n", "no output"); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("content over a stale note: %v", got)
	}
}

// The detail view's three sections: metrics unclipped to terminal width, the
// tailed log, and the footer naming its own keys.
func TestDashDetailViewRendersMetricsLogAndFooter(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	m := dashModel{
		fleetPath: "fleet.yaml",
		entries:   []dashEntry{{name: "up", kind: fleet.KindDaemon}},
		results: []fleet.NodeResult{{
			Name: "up", Outcome: fleet.OutcomeOK,
			Metrics: metrics.Stats{State: "idle", Runner: "llamacpp", ModelID: "org/qwen"},
		}},
		actions:          make([]dashAction, 1),
		detail:           true,
		detailLogContent: "line one\nline two\n",
		width:            80, height: 24,
	}
	view := m.detailView()
	if !strings.Contains(view, "node: up") {
		t.Errorf("header does not name the node:\n%s", view)
	}
	if !strings.Contains(view, "up  idle") {
		t.Errorf("metrics section missing:\n%s", view)
	}
	if !strings.Contains(view, "line one") || !strings.Contains(view, "line two") {
		t.Errorf("log section missing the tailed lines:\n%s", view)
	}
	if !strings.Contains(view, dashFooterHints(dashDetailKeys, false)) {
		t.Errorf("footer does not name the detail view's keys:\n%s", view)
	}
}

// A node whose last answer was a failure shows its outcome and reason in the
// metrics section, degrading rather than failing to render.
func TestDashDetailViewFailingNode(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	m := dashModel{
		entries: []dashEntry{{name: "down", kind: fleet.KindDaemon}},
		results: []fleet.NodeResult{{Name: "down", Outcome: fleet.OutcomeUnreachable, Err: errors.New("connection refused")}},
		actions: make([]dashAction, 1),
		detail:  true,
		width:   80, height: 24,
	}
	view := m.detailView()
	if !strings.Contains(view, "down  unreachable") || !strings.Contains(view, "connection refused") {
		t.Errorf("failing node metrics section missing:\n%s", view)
	}
}

// A node with no log yet shows the same explanation fleet logs gives for it,
// not an empty pane.
func TestDashDetailViewNoLogYet(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	m := dashModel{
		entries:       []dashEntry{{name: "n", kind: fleet.KindDaemon}},
		results:       []fleet.NodeResult{{Name: "n"}},
		actions:       make([]dashAction, 1),
		detail:        true,
		detailLogNote: "no engine log — nothing has run here yet",
		width:         80, height: 24,
	}
	view := m.detailView()
	if !strings.Contains(view, "no engine log — nothing has run here yet") {
		t.Errorf("log note missing:\n%s", view)
	}
}

// An action in flight on the node in view replaces the metrics section with
// its verb and latest status line, the same wording the tile uses.
func TestDashDetailViewActionInFlight(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	m := dashModel{
		entries: []dashEntry{{name: "n", kind: fleet.KindDaemon}},
		results: []fleet.NodeResult{{Name: "n"}},
		actions: []dashAction{{verb: "start", line: "instance starting; retrying in 5s"}},
		detail:  true,
		width:   80, height: 24,
	}
	view := m.detailView()
	if !strings.Contains(view, "n  starting") || !strings.Contains(view, "instance starting; retrying in 5s") {
		t.Errorf("in-flight metrics section missing:\n%s", view)
	}
}

// End to end: entering the detail view shows the node's log and follows it,
// starting from inside the view brings the node up on the normal refresh,
// and escape returns to the grid with the selection intact.
func TestDashProgramDetailViewLogAndBack(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	refreshInterval := dashboardRefreshInterval
	dashboardRefreshInterval = 15 * time.Millisecond
	defer func() { dashboardRefreshInterval = refreshInterval }()
	logInterval := detailLogInterval
	detailLogInterval = 15 * time.Millisecond
	defer func() { detailLogInterval = logInterval }()

	node := newFakeDashNode("stopped")
	node.appendLog("engine booted\n")
	m := &dashModel{
		fleetPath: "fleet.yaml",
		entries:   []dashEntry{{name: "alpha", kind: fleet.KindDaemon, node: node}},
		results:   []fleet.NodeResult{{Name: "alpha"}},
		actions:   make([]dashAction, 1),
	}
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))
	out := tm.Output()
	seen := func(what string, d time.Duration) {
		teatest.WaitFor(t, out, func(b []byte) bool { return bytes.Contains(b, []byte(what)) }, teatest.WithDuration(d))
	}
	seen("stopped", 5*time.Second)
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	seen("engine booted", 5*time.Second)
	node.appendLog("engine ready\n")
	seen("engine ready", 5*time.Second)
	tm.Type("s")
	seen("alpha  running", 5*time.Second)
	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
	seen("↑↓←→ move", 5*time.Second)
	tm.Type("q")
	tm.WaitFinished(t)
	if node.starts != 1 {
		t.Fatalf("starts=%d, want 1", node.starts)
	}
}
