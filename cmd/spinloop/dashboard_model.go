// The `fleet dashboard` model: what the screen shows and how keys and
// messages change it. Bubble Tea drives it through Init/Update/View, but
// every rule here — refresh, actions, layout — is plain Go over plain data,
// so the suite runs the whole logic without a terminal.

package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spinloop-ai/spinloop/internal/daemon"
	"github.com/spinloop-ai/spinloop/internal/fleet"
)

// dashboardRefreshInterval is how often the board re-reads the local daemon
// machines. It is a variable so a test never waits for a slow node.
var dashboardRefreshInterval = 2 * time.Second

// dashboardRemoteRefreshInterval is the cadence for kind: remote
// environments instead. Each of their statuses is a signed call through the
// cloud control plane — a Lambda invocation, not a socket on the
// sideboard — so the board refreshes the local machines on the tick and the
// cloud environments on this slower deadline. Variable, for the same reason.
var dashboardRemoteRefreshInterval = 60 * time.Second

// dashEntry is one fleet-file entry and what it resolved to. Node is nil
// when the entry could not become a node at all — its token reference names
// a variable that is set nowhere — in which case Standing is the outcome its
// panel shows for the life of the view: the very reason the one-shot
// surfaces print for it, so the board never hides a broken node. Kind is the
// fleet file's own, and it decides the refresh cadence, not the node.
type dashEntry struct {
	name     string
	kind     string
	node     fleet.Node
	standing fleet.NodeResult
}

// dashAction is the board's account of a start or stop in flight on one
// node: the verb, the last line the call has reported, and the call's own
// context — the abort's door. A node with nothing in flight carries the zero
// value.
type dashAction struct {
	verb    string             // "start" or "stop"
	line    string             // the call's latest status line; empty until it reports one
	cancel  context.CancelFunc // end the wait on the call; nil where the zero value sits
	aborted bool               // the operator ended the wait, so the line says so
}

// dashModel is the program's state: the board, the selection, and the in-
// flight things — a refresh round per node group (local machines, cloud
// environments), a stop's confirmation, a start or stop per node.
type dashModel struct {
	fleetPath string
	entries   []dashEntry
	results   []fleet.NodeResult // parallel to entries; zero until a round lands
	actions   []dashAction       // parallel to entries; a node's in-flight start or stop

	cursor    int // the selected node
	scrollRow int // the first grid row on screen

	confirm    bool // a stop is waiting on its confirmation
	statusLine string

	// send feeds a message back into the program from outside the Update
	// loop — the in-flight progress of a start, which its call reports from
	// its own goroutine. It is the tea.Program's Send, safe from any
	// goroutine and a no-op once the program has left; nil where the model
	// is driven directly and nothing is wired to the program.
	send func(msg tea.Msg)

	fastGen, slowGen   int       // rounds started in each group; each answer carries its number
	fastBusy, slowBusy bool      // a round in flight in that group
	nextSlowAt         time.Time // when the cloud group is next due; zero means due now

	width, height int

	// detail is the full-screen view of the node under the cursor, opened by
	// enter and closed by escape. The cursor never moves while it is open, so
	// the node in view is always entries[cursor]; detail carries only whether
	// the view is open and the state of its own log tail.
	detail           bool
	detailLogGen     int    // bumped on every open, so a reply from a closed or superseded view is discarded
	detailLogBusy    bool   // a log round is in flight for the node in view
	detailLogFollow  bool   // whether the tick is allowed to start a round; f toggles it
	detailLogOffset  int64  // where the next log round resumes from
	detailLogContent string // the tailed content, trimmed to what the pane can show
	detailLogNote    string // why the pane has no content — empty once it does
}

// dashTickMsg fires on the fast interval.
type dashTickMsg time.Time

// dashRefreshMsg is one completed round of one group. idx and results are
// parallel: the entries this round re-read, and what each answered. An
// answer whose round is no longer the newest in its group is dropped: a late
// reply must not paint over a fresher board.
type dashRefreshMsg struct {
	remote  bool // the cloud group's round
	gen     int
	idx     []int
	results []fleet.NodeResult
}

// dashVerbProgress is the in-flight wording for a verb — the footer and the
// tile share it, since start simply takes -ing and stop drops its p.
func dashVerbProgress(verb string) string {
	if verb == "stop" {
		return "stopping"
	}
	return verb + "ing"
}

// dashActionProgressMsg is one intermediate line of a call behind an in-
// flight action, sent to the program by the call's own goroutine as the work
// proceeds.
type dashActionProgressMsg struct {
	node string
	line string
}

// dashActionMsg is one completed start or stop.
type dashActionMsg struct {
	node   string
	verb   string // "start" or "stop"
	status daemon.StatusResponse
	err    error
}

// dashTickCmd schedules the next fast tick. The tick is one-shot, so every
// tick reschedules the next: drop the reschedule and the board is still
// after the second round.
func dashTickCmd() tea.Cmd {
	return tea.Tick(dashboardRefreshInterval, func(time.Time) tea.Msg { return dashTickMsg{} })
}

// Init starts the rounds the board is due for — the local round plus the
// cloud round, because on a cold open the cloud deadline has never been
// spent — and the tick that keeps them coming.
func (m *dashModel) Init() tea.Cmd {
	return tea.Batch(append([]tea.Cmd{dashTickCmd()}, m.startRounds()...)...)
}

func (m *dashModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.keepVisible()
	case dashTickMsg:
		// One tick, the due rounds. The tick reschedules itself whenever it
		// fires, and a round starts in a group only when none is in flight
		// there, so a slow node stretches one round to its deadline rather
		// than overlapping the next.
		return m, tea.Batch(append([]tea.Cmd{dashTickCmd()}, m.startRounds()...)...)
	case dashRefreshMsg:
		if msg.remote {
			if msg.gen != m.slowGen {
				return m, nil
			}
			m.slowBusy = false
		} else {
			if msg.gen != m.fastGen {
				return m, nil
			}
			m.fastBusy = false
		}
		for i, idx := range msg.idx {
			m.results[idx] = msg.results[i]
		}
	case dashActionProgressMsg:
		if i := m.indexOf(msg.node); i >= 0 && m.actions[i].verb != "" {
			m.actions[i].line = msg.line
		}
	case dashActionMsg:
		aborted := false
		if i := m.indexOf(msg.node); i >= 0 {
			aborted = m.actions[i].aborted
			m.actions[i] = dashAction{}
		}
		m.statusLine = dashActionLine(msg, aborted)
	case detailLogTickMsg:
		// The tick reschedules itself only while the view is open on a node
		// that could ever answer; closing it, or a switch to a standing node
		// that opened without ever scheduling this chain, is the shutdown
		// path — the next tick simply stops rather than resurrecting it.
		if !m.detail || m.entries[m.cursor].node == nil {
			return m, nil
		}
		// The chain keeps ticking whether or not follow is on — pausing
		// only skips the round it would have started, so unpausing with f
		// needs nothing more than flipping the flag back.
		var cmd tea.Cmd
		if m.detailLogFollow {
			cmd = m.startDetailLogRound()
		}
		return m, tea.Batch(detailLogTickCmd(), cmd)
	case dashDetailLogMsg:
		m.detailLogBusy = false
		if !m.detail || msg.gen != m.detailLogGen {
			return m, nil
		}
		m.applyDetailLog(msg.result)
	case tea.KeyMsg:
		if m.confirm {
			switch msg.String() {
			case "y":
				return m, m.beginAction("stop")
			case "n", "esc":
				m.confirm = false
			case "q", "ctrl+c":
				m.confirm = false
				return m, tea.Quit
			}
			return m, nil
		}
		if m.detail {
			return m, m.updateDetailKey(msg)
		}
		var cmd tea.Cmd
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "enter":
			if len(m.entries) > 0 {
				cmd = m.openDetail()
			}
		case "down":
			cols := dashCols(m.effWidth())
			if len(m.entries) > 0 {
				if next := m.cursor + cols; next < len(m.entries) {
					m.cursor = next
				} else if curRow, lastRow := m.cursor/cols, (len(m.entries)-1)/cols; curRow < lastRow {
					// The row below exists but is shorter than the grid's
					// width and does not reach this column: land on the
					// last tile that row has, rather than the row itself
					// having nothing to move onto.
					m.cursor = len(m.entries) - 1
				}
				// Otherwise the current row is already the last one: no
				// row below to move onto, so the selection stays put
				// rather than jumping sideways to a different column.
			}
			m.keepVisible()
		case "up":
			cols := dashCols(m.effWidth())
			// Every row above the last is fully populated, so a row above
			// always has a tile in the same column; where there is no row
			// above, the selection stays put rather than jumping to column
			// 0.
			if next := m.cursor - cols; next >= 0 {
				m.cursor = next
			}
			m.keepVisible()
		case "right":
			cols := dashCols(m.effWidth())
			if next := m.cursor + 1; m.cursor%cols+1 < cols && next < len(m.entries) {
				m.cursor = next
			}
			m.keepVisible()
		case "left":
			cols := dashCols(m.effWidth())
			if m.cursor%cols != 0 {
				m.cursor--
			}
			m.keepVisible()
		case "s":
			if len(m.entries) > 0 && m.actions[m.cursor].verb == "" {
				cmd = m.beginAction("start")
			}
		case "a":
			// End the wait on the node's in-flight action — the one-shot
			// equivalent of Ctrl+C. A node with nothing in flight is driven
			// by nothing.
			if len(m.entries) > 0 {
				m.abortAction()
			}
		case "x":
			if len(m.entries) > 0 && m.actions[m.cursor].verb == "" {
				m.confirm = true
			}
		case "r":
			// A manual refresh is due for every node, cloud or local,
			// whatever their own deadlines say.
			m.nextSlowAt = time.Time{}
			cmds := m.startRounds()
			if len(cmds) > 0 {
				cmd = tea.Batch(cmds...)
			}
		case "pgup":
			m.scrollRow = dashClampScroll(m.scrollRow-dashVisibleRows(m.effHeight()), len(m.entries), m.effWidth(), m.effHeight())
		case "pgdown":
			m.scrollRow = dashClampScroll(m.scrollRow+dashVisibleRows(m.effHeight()), len(m.entries), m.effWidth(), m.effHeight())
		}
		return m, cmd
	}
	return m, nil
}

// startRounds starts the rounds the board is due for: the local machines
// whenever the tick fires, the cloud environments when their own deadline
// has passed. A round is never started over one still in flight in the same
// group, and a group with nothing to read starts nothing.
func (m *dashModel) startRounds() []tea.Cmd {
	var cmds []tea.Cmd
	if cmd := m.refreshRemoteGroup(false); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if !time.Now().Before(m.nextSlowAt) {
		if cmd := m.refreshRemoteGroup(true); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return cmds
}

// refreshRemoteGroup starts one round over one group of live nodes — the
// local daemon machines (remote false) or the cloud environments (remote
// true) — and returns the round's command, or nil when the group is empty or
// already has a round in flight. Starting the cloud round spends its
// deadline: it moves to one interval away, so the next due round is a
// whole interval later.
//
// The round carries a context with a deadline of its group's interval, so a
// node slower than the cadence shows an outcome this round and gets its turn
// again next. Each node answers independently — the fan-out calls them
// concurrently — so one slow node delays no other, and a slow cloud round
// stretches only its own group.
func (m *dashModel) refreshRemoteGroup(remote bool) tea.Cmd {
	idx := make([]int, 0, len(m.entries))
	nodes := make([]fleet.Node, 0, len(m.entries))
	for i, e := range m.entries {
		if e.node != nil && (e.kind == fleet.KindRemote) == remote {
			idx = append(idx, i)
			nodes = append(nodes, e.node)
		}
	}
	if len(nodes) == 0 {
		return nil
	}
	if remote {
		if m.slowBusy {
			return nil
		}
		m.slowBusy = true
		m.slowGen++
		m.nextSlowAt = time.Now().Add(dashboardRemoteRefreshInterval)
	} else {
		if m.fastBusy {
			return nil
		}
		m.fastBusy = true
		m.fastGen++
	}
	gen := m.generationFor(remote)
	interval := m.intervalFor(remote)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), interval)
		defer cancel()
		return dashRefreshMsg{remote: remote, gen: gen, idx: idx, results: fleet.FanOutNodes(ctx, fleet.MetricsCall, nodes)}
	}
}

func (m *dashModel) generationFor(remote bool) int {
	if remote {
		return m.slowGen
	}
	return m.fastGen
}

func (m *dashModel) intervalFor(remote bool) time.Duration {
	if remote {
		return dashboardRemoteRefreshInterval
	}
	return dashboardRefreshInterval
}

// dashActionLine is the footer's account of a finished action, on the same
// result the fleet fan-out gives the one-shot commands, so the board cannot
// claim a start worked when the row of `fleet start` would not. A failure the
// operator's abort caused is not reported as a failure of the node — the wait
// was abandoned, and a success that races the abort is still reported as the
// success, since the line is about what the node did, not what the dashboard
// did.
func dashActionLine(msg dashActionMsg, aborted bool) string {
	r := fleet.Result(msg.node, msg.err, msg.status)
	if !r.OK() {
		if aborted {
			return msg.node + ": " + msg.verb + " abandoned"
		}
		return msg.node + ": " + msg.verb + " failed — " + r.Detail()
	}
	state := msg.status.State
	if state == "" {
		state = "done"
	}
	return msg.node + ": " + msg.verb + " — " + state
}

// beginAction sets off a start or stop on the selected node. The action runs
// in a Bubble Tea command — its own goroutine — and carries no deadline: a
// cloud instance waking takes minutes, and a deadline would report a failure
// to what is a slow success. The action is one per node, never one per board:
// another node starts the same instant this one is still waking, and each
// call reports itself on its own tile through the progress door, so the
// footer — one line — keeps only the final outcomes. While it flies, the
// refresh rounds keep coming and the panel carries their answers beside the
// call's own lines — the node's report is the node's account of the state;
// when the call returns, the panel is that report alone. The action runs on
// its own context, held beside it on the board — the operator's abort is
// that context's cancellation, not a timer.
func (m *dashModel) beginAction(verb string) tea.Cmd {
	e := m.entries[m.cursor]
	m.confirm = false
	if e.node == nil {
		m.statusLine = e.name + ": " + e.standing.Detail()
		return nil
	}
	if m.actions[m.cursor].verb != "" {
		m.statusLine = e.name + ": still " + dashVerbProgress(m.actions[m.cursor].verb)
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.actions[m.cursor] = dashAction{verb: verb, cancel: cancel}
	m.statusLine = dashVerbProgress(verb) + " " + e.name + "…"
	send := m.send
	progress := func(line string) {
		if send != nil {
			send(dashActionProgressMsg{node: e.name, line: line})
		}
	}
	starter, isStarter := e.node.(fleet.ProgressStarter)
	var act func(context.Context) (daemon.StatusResponse, error)
	switch {
	case verb == "stop":
		act = e.node.Stop
	case isStarter:
		act = func(ctx context.Context) (daemon.StatusResponse, error) {
			return starter.StartWithProgress(ctx, progress)
		}
	default:
		act = e.node.Start
	}
	return func() tea.Msg {
		status, err := act(ctx)
		cancel()
		return dashActionMsg{node: e.name, verb: verb, status: status, err: err}
	}
}

// abortAction ends the wait on the selected node's in-flight start. Only a
// start is abortable: it is the one action with no deadline of its own — a
// cold cloud wake takes minutes, so a wait the operator no longer wants to
// watch is a wait only the operator can end. A stop is not: it targets a
// node that is already running, its own call carries no comparable open-
// ended wait, and abandoning the wait on it would leave the operator unsure
// whether the stop still went ahead — so a stop in flight, or no action at
// all, drives nothing here. The abort ends the wait, not the work: the
// call's own loop returns on the done context — at the retry wait or mid-
// request, on the path it already has for a given-up wait — and its final
// message lands as for any finished action: the tile clears, the node may
// be started or stopped again, and the line says the wait was abandoned,
// never that a wake the cloud is carrying was cancelled. What the wake goes
// on to do comes back on the next refresh.
func (m *dashModel) abortAction() {
	i := m.cursor
	if m.actions[i].verb != "start" {
		return
	}
	m.actions[i].aborted = true
	if cancel := m.actions[i].cancel; cancel != nil {
		cancel()
	}
}

// canAbort reports whether the abort key would do anything for the node
// under the cursor: only a start in flight is abortable, so an idle,
// running, or already-stopping node has nothing to abort. The footer uses
// this to drop the abort hint rather than advertise a key that changes
// nothing for the node it describes.
func (m dashModel) canAbort() bool {
	return len(m.entries) > 0 && m.actions[m.cursor].verb == "start"
}

// indexOf finds an entry by name. Fleet-file names are unique — the fleet
// file itself refuses a repeat — so a find is a position, not a set.
func (m *dashModel) indexOf(name string) int {
	for i, e := range m.entries {
		if e.name == name {
			return i
		}
	}
	return -1
}

// keepVisible scrolls so the selected tile lies in the visible rows.
func (m *dashModel) keepVisible() {
	if len(m.entries) == 0 {
		m.scrollRow = 0
		return
	}
	row := m.cursor / dashCols(m.effWidth())
	avail := dashVisibleRows(m.effHeight())
	if row < m.scrollRow || row >= m.scrollRow+avail {
		m.scrollRow = row
	}
	m.scrollRow = dashClampScroll(m.scrollRow, len(m.entries), m.effWidth(), m.effHeight())
}

// effWidth and effHeight report the window, defaulting where none was ever
// reported: Bubble Tea measures the real screen at startup, so a zero can
// only mean not-measured, not a real size.
func (m dashModel) effWidth() int {
	if m.width < 1 {
		return 80
	}
	return m.width
}

func (m dashModel) effHeight() int {
	if m.height < 1 {
		return 24
	}
	return m.height
}

// View draws the frame: the header, the visible grid rows, the footer.
func (m dashModel) View() string {
	if m.detail {
		return m.detailView()
	}
	w, h := m.effWidth(), m.effHeight()
	tiles := make([]string, len(m.entries))
	for i := range m.entries {
		tiles[i] = dashTile(m.entries[i].name, m.results[i], i == m.cursor, m.actions[i])
	}
	rows := dashGridRows(tiles, dashCols(w))
	lo := m.scrollRow
	if lo > len(rows) {
		lo = len(rows)
	}
	hi := lo + dashVisibleRows(h)
	if hi > len(rows) {
		hi = len(rows)
	}
	parts := []string{m.headerLine(w)}
	if hi > lo {
		parts = append(parts, strings.Join(rows[lo:hi], "\n"))
	}
	parts = append(parts, m.footerLine(w, dashFooterHints(dashGridKeys, m.canAbort())))
	return strings.Join(parts, "\n")
}

func (m dashModel) headerLine(w int) string {
	word := "nodes"
	if len(m.entries) == 1 {
		word = "node"
	}
	return dashClip(fmt.Sprintf("fleet dashboard  %s  (%d %s)", m.fleetPath, len(m.entries), word), w)
}

// dashGridKeys is the grid's own key help; the detail view's footer shares
// footerLine but names its own keys instead (see dashDetailKeys).
const dashGridKeys = "↑↓←→ move   s start   a abort   x stop   r refresh   q quit"

// footerLine is the frame's bottom line: the given key help, replaced by the
// stop confirmation prompt while one is pending, with the status line and a
// "refreshing" marker appended — shared by the grid and the detail view so
// the two cannot word the confirmation or a status outcome differently.
func (m dashModel) footerLine(w int, keys string) string {
	line := keys
	if m.confirm && len(m.entries) > 0 {
		line = "stop " + m.entries[m.cursor].name + "?   y yes   n no"
	}
	if m.statusLine != "" {
		line += "   " + m.statusLine
	}
	if m.fastBusy || m.slowBusy {
		line += "   refreshing"
	}
	return dashClip(line, w)
}
