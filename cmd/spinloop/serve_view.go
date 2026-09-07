// The serve view: what `spinloop serve` draws on a terminal — the engine's
// metrics above, its tailed log below, and a footer naming the keys the view
// answers to. The frame is the fleet dashboard's node detail screen three-
// section layout: the metrics section is the same lines the detail view and
// the metrics formats print for the same reading — state with uptime, what
// is served, the last-active line, the resource series, the token counters —
// with each resource series drawn in both formats at once, the gauge of the
// current reading and, beneath it, the bar of the retained history. The log
// section follows the engine's own log the way the detail view follows its
// node's. Everything it reads is in-process: the daemon the serve process
// runs itself, not the network. Bubble Tea drives the model through
// Init/Update/View, but every rule here is plain Go over plain data, so the
// suite runs the whole logic without a terminal.

package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spinloop-ai/spinloop/internal/daemon"
	"github.com/spinloop-ai/spinloop/internal/metrics"
	"github.com/spinloop-ai/spinloop/internal/spinloop"
)

// newServeProgram builds the program the view runs on: the model, the
// alternate screen, and nothing else. A variable so a test runs the view with
// injected input and output, where opening the terminal's own input would
// fail.
var newServeProgram = func(m tea.Model, opts ...tea.ProgramOption) *tea.Program {
	return tea.NewProgram(m, append([]tea.ProgramOption{tea.WithAltScreen()}, opts...)...)
}

// serveViewLogBudget is how many lines of the tailed log the view retains in
// memory. A few thousand is well past any pane the frame can show, so the
// scroll can reach back as far as the frame can ever go; the file on disk
// keeps the whole record, so the budget only bounds the copy.
const serveViewLogBudget = 2000

// serveViewLogTailBytes is the byte budget of the first read of a freshly
// opened view — the backlog — the same way the detail view budgets its tail.
const serveViewLogTailBytes = 200 * bytesPerLineGuess

// serveViewMetricsTimeout caps one in-process metrics read. The read takes
// host stats and the sampler's own counters — no network — so a deadline this
// loose only catches a wedged host probe, which is exactly what the age
// beside a stale reading is for.
const serveViewMetricsTimeout = 5 * time.Second

// serveViewKeys is the view's footer key help: exactly the keys the view
// answers to — the scroll, the follow, and the quit — and nothing the view
// cannot do. Starting, stopping, keeping and aborting are not among them:
// the engine is serve's own, and leaving is what stops it.
const serveViewKeys = "↑↓ scroll   pgup/pgdown page   f follow   q quit"

// serveView is the program's state: the metrics reading and when it was
// taken, the tailed log and where its window sits in it, and the window size
// the frame draws to.
type serveView struct {
	spinloopPath string

	stats   metrics.Stats // the last successful read; zero until the first
	statsAt time.Time     // when it was taken; a failed read leaves the last one here, and its age is drawn from it

	logOffset  int64  // where the next poll resumes from; TailLog for the first, which is the backlog
	logContent string // the tailed lines, most recent last, trimmed to the budget
	logBudget  int    // how many lines the tail retains
	logBehind  int    // how many lines the window's bottom sits behind the newest; 0 is on the newest line
	logFollow  bool   // whether the poll picks up new output; f pauses and resumes it
	logBusy    bool   // a poll is in flight
	logGen     int    // bumped to supersede a poll in flight
	logNote    string // why the pane has no content — empty once it does, or once a read fails with content in place

	metricsBusy bool
	metricsGen  int

	width, height int

	// send feeds a message back into the program from outside the Update
	// loop — the engine's own exit, which its wait goroutine reports. It is
	// the tea.Program's Send, safe from any goroutine and a no-op once the
	// program has left; nil where the model is driven directly in a test.
	send func(tea.Msg)
	// stop ends the run: the graceful stop, escalating as a stop does
	// elsewhere. It blocks until the engine is down, so it runs in a
	// Bubble Tea command, never inside Update.
	stop func()
	// readMetrics and readLog are the view's two data paths, injected so a
	// test drives the model with its own answers. The real ones call the
	// in-process daemon and the daemon's own log read — the same functions
	// the control API's handlers call, so the view and the API cannot
	// report different facts about the same engine.
	readMetrics func() (metrics.Stats, error)
	readLog     func(offset int64, limit int) (daemon.LogsResponse, error)
}

// serveViewMetricsTickMsg fires on the dashboard's own local cadence.
type serveViewMetricsTickMsg time.Time

func serveViewMetricsTickCmd() tea.Cmd {
	return tea.Tick(dashboardRefreshInterval, func(time.Time) tea.Msg { return serveViewMetricsTickMsg{} })
}

// serveViewLogTickMsg fires on the detail view's log cadence, the chain the
// log poll runs on, separately from the metrics tick.
type serveViewLogTickMsg time.Time

func serveViewLogTickCmd() tea.Cmd {
	return tea.Tick(detailLogInterval, func(time.Time) tea.Msg { return serveViewLogTickMsg{} })
}

// serveViewMetricsMsg is one completed in-process reading. gen ties it to
// the model that started it, so a reply a superseded read sent is discarded
// rather than painted over a newer one.
type serveViewMetricsMsg struct {
	gen   int
	stats metrics.Stats
	err   error
}

// serveViewLogMsg is one completed poll of the engine's log.
type serveViewLogMsg struct {
	gen   int
	reply daemon.LogsResponse
	err   error
}

// serveViewStoppedMsg says the operator's stop has landed and the engine is
// down: the view quits on it.
type serveViewStoppedMsg struct{}

// serveViewEngineExitedMsg says the engine exited on its own, whatever the
// cause: the view quits on it, and the run reports the engine's exit status
// after the program has left.
type serveViewEngineExitedMsg struct{}

// Init starts both chains at once — the first metrics read and the first
// log poll, which is the backlog, because the offset opens on the tail.
func (m *serveView) Init() tea.Cmd {
	return tea.Batch(
		serveViewMetricsTickCmd(), m.startMetricsRead(),
		serveViewLogTickCmd(), m.startLogPoll(),
	)
}

func (m *serveView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.clampBehind()
	case serveViewMetricsTickMsg:
		// One tick, one read, the tick rescheduling itself for the life of
		// the view. A read starts only when none is in flight, so a slow
		// read stretches to its own time rather than overlapping the next.
		return m, tea.Batch(serveViewMetricsTickCmd(), m.startMetricsRead())
	case serveViewMetricsMsg:
		m.metricsBusy = false
		if msg.gen != m.metricsGen {
			return m, nil
		}
		if msg.err == nil {
			// A successful read replaces the one on screen. A failed one
			// leaves the last in place: its statsAt stands, and the age
			// beside the state line says what it says.
			m.stats = msg.stats
			m.statsAt = dashNow()
		}
	case serveViewLogTickMsg:
		// The chain keeps ticking whether or not follow is on — pausing
		// only skips the round it would have started, so unpausing with f
		// needs nothing more than flipping the flag back, and the metrics
		// section's own refresh never felt it either.
		var cmd tea.Cmd
		if m.logFollow {
			cmd = m.startLogPoll()
		}
		return m, tea.Batch(serveViewLogTickCmd(), cmd)
	case serveViewLogMsg:
		m.logBusy = false
		if msg.gen != m.logGen {
			return m, nil
		}
		m.applyLogReply(msg)
	case serveViewStoppedMsg, serveViewEngineExitedMsg:
		return m, tea.Quit
	case tea.KeyMsg:
		return m, m.updateKey(msg)
	}
	return m, nil
}

// updateKey answers the keys the view reads: the scroll of the log pane, f
// to pause and resume the follow, and q or Ctrl+C to leave — which stops the
// engine. Nothing else drives anything in the view, so nothing else is read.
func (m *serveView) updateKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "q", "ctrl+c":
		return m.stopCmd()
	case "f":
		m.logFollow = !m.logFollow
	case "up":
		m.logBehind++
		m.clampBehind()
	case "down":
		m.logBehind--
		m.clampBehind()
	case "pgup":
		m.logBehind += m.logPaneRows()
		m.clampBehind()
	case "pgdown":
		m.logBehind -= m.logPaneRows()
		m.clampBehind()
	}
	return nil
}

// stopCmd ends the run. The stop blocks until the engine is down — the
// graceful escalation a stop does elsewhere — and the view quits on the round
// trip, so the screen closes when the engine is actually stopped, not before.
func (m *serveView) stopCmd() tea.Cmd {
	return func() tea.Msg {
		if m.stop != nil {
			m.stop()
		}
		return serveViewStoppedMsg{}
	}
}

// startMetricsRead takes one in-process reading through the injected read.
// It never starts a second read over one still in flight.
func (m *serveView) startMetricsRead() tea.Cmd {
	if m.metricsBusy || m.readMetrics == nil {
		return nil
	}
	m.metricsBusy = true
	gen := m.metricsGen
	read := m.readMetrics
	return func() tea.Msg {
		stats, err := read()
		return serveViewMetricsMsg{gen: gen, stats: stats, err: err}
	}
}

// startLogPoll reads the engine's log through the daemon's own log read:
// the tail on the first read, the stored offset on every one after, the
// daemon's byte bounds doing the rest. It never starts a second poll over
// one still in flight.
func (m *serveView) startLogPoll() tea.Cmd {
	if m.logBusy || m.readLog == nil {
		return nil
	}
	m.logBusy = true
	gen := m.logGen
	offset, limit := m.logOffset, 0
	if offset == daemon.TailLog {
		limit = serveViewLogTailBytes
	}
	read := m.readLog
	return func() tea.Msg {
		reply, err := read(offset, limit)
		return serveViewLogMsg{gen: gen, reply: reply, err: err}
	}
}

// applyLogReply folds one completed poll into the view. New output is
// appended and trimmed to the line budget, so a long session never grows an
// unbounded buffer — the file on disk keeps the whole record — and a failed
// poll leaves the prior content in place rather than blanking it. The
// window's stick-and-stay rule is arithmetic on logBehind: on the newest
// line new output keeps it there, scrolled away it stays put as the new
// lines land behind it.
func (m *serveView) applyLogReply(msg serveViewLogMsg) {
	if msg.err != nil {
		if m.logContent == "" {
			m.logNote = "log read failed: " + msg.err.Error()
		}
		return
	}
	m.logNote = ""
	if msg.reply.StaleOffset {
		// The file shrank — truncated or replaced. The content in memory is
		// from the file it replaced, so it goes, and the cursor resumes from
		// the reply's end, the rule the log read defines.
		m.logContent = ""
		m.logBehind = 0
		m.logOffset = msg.reply.NextOffset
		return
	}
	m.logOffset = msg.reply.NextOffset
	if msg.reply.Content != "" {
		appended := lineCount(msg.reply.Content)
		m.logContent = lastLines(m.logContent+msg.reply.Content, m.logBudget)
		if m.logBehind > 0 {
			m.logBehind += appended
		}
		m.clampBehind()
	}
}

// lineCount is how many lines s carries, the way the pane counts them: the
// trailing newline does not start a line.
func lineCount(s string) int {
	if s == "" {
		return 0
	}
	return len(strings.Split(strings.TrimRight(s, "\n"), "\n"))
}

// clampBehind keeps the window inside the retained content: it never shows
// past the oldest retained line or ahead of the newest, so a press at either
// end leaves it where it is.
func (m *serveView) clampBehind() {
	if m.logBehind < 0 {
		m.logBehind = 0
		return
	}
	max := lineCount(m.logContent) - m.logPaneRows()
	if max < 0 {
		max = 0
	}
	if m.logBehind > max {
		m.logBehind = max
	}
}

// sectionHeights splits the frame's rows between the metrics section (its
// natural length) and the log section (whatever remains after the header,
// the footer, and the three dividers around the three sections), floored at
// one row each — the detail view's own split.
func (m *serveView) sectionHeights() (metricsH, logH int) {
	metricsH = len(m.metricsLines())
	if metricsH < 1 {
		metricsH = 1
	}
	const fixedRows = 5 // header + divider + divider + divider + footer
	logH = m.effHeight() - fixedRows - metricsH
	if logH < 1 {
		logH = 1
	}
	return metricsH, logH
}

// logPaneRows is how many rows of log the frame shows at once — the page
// size pgup and pgdown move by.
func (m *serveView) logPaneRows() int {
	_, logH := m.sectionHeights()
	return logH
}

// effWidth and effHeight report the window, defaulting where none was ever
// reported: Bubble Tea measures the real screen at startup, so a zero can
// only mean not-measured, not a real size.
func (m serveView) effWidth() int {
	if m.width < 1 {
		return 80
	}
	return m.width
}

func (m serveView) effHeight() int {
	if m.height < 1 {
		return 24
	}
	return m.height
}

// metricsLines is the metrics section: the same lines the dashboard's detail
// screen and the metrics formats print for the reading — state with uptime,
// what is served, the last-active line, the resource series in both formats
// at once, the token counters — so the view cannot word a number the other
// surfaces would not.
func (m serveView) metricsLines() []string {
	if m.statsAt.IsZero() {
		return []string{"waiting for the first reading…"}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s%s\n", dashStateLine(m.stats.State, m.stats), m.staleSuffix())
	if line := dashTileServingLine(m.stats); line != "" {
		fmt.Fprintln(&b, line)
	}
	renderActiveIndented(&b, m.stats.LastActiveAt, m.stats.IdleSeconds, m.stats.RetainUntil, dashNow())
	// A running reading carries its current figures; a settled one carries
	// its retained history, which survives a stop — the detail view's own
	// rule for when the series exist at all.
	resources := m.stats.State == "running" || len(m.stats.History) > 0
	if resources {
		renderStatCombined(&b, m.stats.CPU, m.stats.Memory, m.stats.GPUs, m.stats.History)
		renderTokenLines(&b, m.stats.Tokens)
	}
	lines := strings.Split(b.String(), "\n")
	return lines[:len(lines)-1]
}

// staleSuffix is the "· 3m ago" the state line carries once the reading has
// aged past the dashboard's own stale rule, or "" while it is current: a
// reading the view could not renew is not drawn identically to one just
// read.
func (m serveView) staleSuffix() string {
	if m.statsAt.IsZero() {
		return ""
	}
	staleAfter := dashStaleThreshold * dashboardRefreshInterval
	age := dashNow().Sub(m.statsAt)
	if age < staleAfter {
		return ""
	}
	return "  · " + formatDuration(int(age.Seconds())) + " ago"
}

// View draws the frame: the title bar — the screen, the Spinloop's path, and
// the log's following or paused state to the right — the metrics section, a
// divider, the tailed log filling the remaining rows, a divider, and the
// footer naming the view's keys.
func (m serveView) View() string {
	w := m.effWidth()
	metricsH, avail := m.sectionHeights()

	logState := "following"
	if !m.logFollow {
		logState = "paused"
	}
	header := dashTitleBar("serve", m.spinloopPath+"   log: "+logState, w)
	divider := strings.Repeat("─", w)

	metrics := m.metricsLines()
	for len(metrics) < metricsH {
		metrics = append(metrics, "")
	}

	logLines := detailLogLines(m.logContent, m.logNote)
	if m.logBehind > 0 {
		// Scrolled away: the window ends that many lines behind the newest.
		end := len(logLines) - m.logBehind
		if end < 0 {
			end = 0
		}
		logLines = logLines[:end]
	}
	if len(logLines) > avail {
		logLines = logLines[len(logLines)-avail:]
	}

	parts := make([]string, 0, len(metrics)+len(logLines)+4)
	parts = append(parts, header, divider)
	for _, line := range metrics {
		parts = append(parts, dashClip(line, w))
	}
	parts = append(parts, divider)
	for _, line := range logLines {
		parts = append(parts, dashClip(line, w))
	}
	parts = append(parts, divider)
	parts = append(parts, dashClip(dashKeyHints(serveViewKeys), w))
	return strings.Join(parts, "\n")
}

// runServeView is `spinloop serve` on a terminal: the engine runs through
// the shared supervised-foreground construction with its output captured to
// the daemon's state-dir engine log, and the view draws on the alternate
// screen for the life of the run. The engine starts before the view opens,
// so a missing binary fails with its install hint around no view, and the
// run reports the engine's exit status whatever closed the view — the
// operator's q or the engine's own exit.
func runServeView(sel spinloop.Selection, spinloopPath string, engine serveEngine, argv []string, apiOn bool, apiAddr, logLevel string) error {
	stateDir, err := daemon.StateDir()
	if err != nil {
		return err
	}
	logPath := filepath.Join(stateDir, "engine.log")
	run, err := startSupervisedForeground(sel, spinloopPath, engine, argv, apiOn, apiAddr, logLevel, logPath)
	if err != nil {
		return err
	}
	m := &serveView{
		spinloopPath: spinloopPath,
		logOffset:    daemon.TailLog,
		logBudget:    serveViewLogBudget,
		logFollow:    true,
		stop:         run.stop,
		readMetrics: func() (metrics.Stats, error) {
			ctx, cancel := context.WithTimeout(context.Background(), serveViewMetricsTimeout)
			defer cancel()
			return run.d.Metrics(ctx), nil
		},
		readLog: func(offset int64, limit int) (daemon.LogsResponse, error) {
			return daemon.ReadLog(logPath, offset, limit)
		},
	}
	prog := newServeProgram(m)
	// The engine's own exit closes the view, whatever its cause. The send is
	// wired before the wait goroutine starts, so it cannot fire into a nil —
	// and it is a no-op once the program has left, so the two exit paths
	// cannot double-quit.
	m.send = prog.Send
	go func() {
		run.wait()
		m.send(serveViewEngineExitedMsg{})
	}()
	if _, err := prog.Run(); err != nil {
		return err
	}
	// The view has left and the engine is down: the run reports the
	// engine's exit status as a foreground serve always has, a stop on
	// request counting as success.
	return run.wait()
}
