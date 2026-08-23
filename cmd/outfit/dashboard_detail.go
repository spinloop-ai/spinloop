// The full-screen node detail view: enter opens it on the node under the
// cursor, replacing the grid with that node's unclipped metrics, its tailed
// engine log, and a footer naming the keys the view answers to; escape
// closes it. The metrics section is dashNodeContentLines — the exact lines
// the tile draws, unclipped — so the two surfaces cannot disagree; the log
// section follows fleet.LogsCall the same way `fleet logs -f` follows one
// node. Everything else about the board — the grid's own refresh, and any
// action already in flight on another node — keeps running behind it; only
// the log poll is specific to the view being open.

package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lucinate-ai/outfit/internal/daemon"
	"github.com/lucinate-ai/outfit/internal/fleet"
)

// detailLogInterval is how often the detail view polls its node's log. A
// variable so a test need not wait on it.
var detailLogInterval = 3 * time.Second

// dashDetailKeys is the detail view's footer key help, sharing footerLine
// with the grid's own (dashGridKeys) so a status outcome or the stop
// confirmation cannot be worded differently between the two.
const dashDetailKeys = "esc back   s start   x stop   a abort   f follow"

// detailLogTickMsg fires on detailLogInterval while the detail view is open.
type detailLogTickMsg time.Time

// detailLogTickCmd schedules the next log poll. Like the grid's own tick, it
// is one-shot and rescheduled by its own handler — here, only while the
// detail view is still open, which is what stops the chain on close.
func detailLogTickCmd() tea.Cmd {
	return tea.Tick(detailLogInterval, func(time.Time) tea.Msg { return detailLogTickMsg{} })
}

// dashDetailLogMsg is one completed poll of the node in view's log. gen ties
// it to the view that started it: a reply from a view since closed, or
// reopened (on the same or a different node), carries the old generation and
// is discarded rather than painted.
type dashDetailLogMsg struct {
	gen    int
	result fleet.NodeResult
}

// openDetail opens the full-screen view on the node under the cursor and
// starts its log tail. The cursor does not move, so the node in view is
// entries[cursor] for as long as the view stays open.
func (m *dashModel) openDetail() tea.Cmd {
	m.detail = true
	m.detailLogGen++
	m.detailLogBusy = false
	m.detailLogFollow = true
	m.detailLogOffset = daemon.TailLog
	m.detailLogContent = ""
	m.detailLogNote = ""

	e := m.entries[m.cursor]
	if e.node == nil {
		// The entry never became a node — there is no log call to make, and
		// the standing outcome is the whole story for the life of the view.
		m.detailLogNote = fmt.Sprintf("%s: %s", e.standing.Outcome, e.standing.Detail())
		return nil
	}
	return tea.Batch(detailLogTickCmd(), m.startDetailLogRound())
}

// updateDetailKey answers the keys the detail view reads: the same
// start/stop/abort the grid applies to the node under the cursor — which is
// the node in view, since the cursor never moves while the view is open —
// escape to close it, and f to pause or resume the log tail. Quit lives on
// the grid only: leaving the dashboard from inside the detail view means
// escaping back to it first, the same as any other nested screen. Navigation
// and the manual refresh are the grid's own keys and are not read here
// either.
func (m *dashModel) updateDetailKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		m.detail = false
	case "f":
		// The polling chain is already running (or already stopped, for a
		// standing node) for the life of the view; flipping the flag is all
		// a resume needs — the next tick picks it straight back up.
		m.detailLogFollow = !m.detailLogFollow
	case "s":
		if len(m.entries) > 0 && m.actions[m.cursor].verb == "" {
			return m.beginAction("start")
		}
	case "a":
		if len(m.entries) > 0 {
			m.abortAction()
		}
	case "x":
		if len(m.entries) > 0 && m.actions[m.cursor].verb == "" {
			m.confirm = true
		}
	}
	return nil
}

// startDetailLogRound polls the node in view's log through the same
// fleet.LogsCall/FanOutNodes seam the grid's metrics rounds use, over the
// single node in view. It never starts a second round over one still in
// flight.
func (m *dashModel) startDetailLogRound() tea.Cmd {
	if m.detailLogBusy {
		return nil
	}
	e := m.entries[m.cursor]
	if e.node == nil {
		return nil
	}
	m.detailLogBusy = true
	gen := m.detailLogGen
	node, offset := e.node, m.detailLogOffset
	limit := 0
	if offset == daemon.TailLog {
		limit = dashDetailLogTailBytes
	}
	return func() tea.Msg {
		// LogsCall keys its offset map by the node's own Name(), not the
		// fleet entry's — the same seam fleet.FanOut itself resumes through.
		offsets := map[string]int64{node.Name(): offset}
		results := fleet.FanOutNodes(context.Background(), fleet.LogsCall(offsets, limit), []fleet.Node{node})
		return dashDetailLogMsg{gen: gen, result: results[0]}
	}
}

// dashDetailLogTailBytes bounds the first read of a freshly opened view, the
// same way fleet logs turns its line limit into a byte budget for the tail.
const dashDetailLogTailBytes = 200 * bytesPerLineGuess

// applyDetailLog folds one completed log round into the view: new content is
// appended and trimmed to what the pane can show, so a long session never
// grows an unbounded buffer, and a failed round leaves prior content in
// place rather than blanking it. The note explains an empty pane — no log
// yet, nothing new to add, or why the round failed — and clears once there
// is content to show instead.
func (m *dashModel) applyDetailLog(r fleet.NodeResult) {
	if r.OK() {
		m.detailLogOffset = r.Logs.NextOffset
		if r.Logs.Content != "" {
			m.detailLogContent = lastLines(m.detailLogContent+r.Logs.Content, m.detailLogCapacity())
		}
	}
	if m.detailLogContent == "" {
		m.detailLogNote = fleetLogsNote(r)
	} else {
		m.detailLogNote = ""
	}
}

// detailLogCapacity is how many lines of log the pane has room for, given
// the metrics section's current height — the same bound applied when
// trimming the accumulated content, so the buffer never holds more than the
// view can ever show.
func (m *dashModel) detailLogCapacity() int {
	_, avail := m.detailSectionHeights()
	return avail
}

// detailSectionHeights splits the frame's rows between the metrics section
// (dashNodeContentLines' natural length for the node in view) and the log
// section (whatever remains after the header, footer, and the three dividers
// around the three sections), floored at one row each.
func (m *dashModel) detailSectionHeights() (metrics, log int) {
	e := m.entries[m.cursor]
	metrics = len(dashNodeContentLines(e.name, m.results[m.cursor], m.actions[m.cursor]))
	if metrics < 1 {
		metrics = 1
	}
	const fixedRows = 5 // header + divider + divider + divider + footer
	log = m.effHeight() - fixedRows - metrics
	if log < 1 {
		log = 1
	}
	return metrics, log
}

// detailLogLines is the log section's content: the tailed lines, most recent
// last, or the standing note when there is nothing to show yet.
func detailLogLines(content, note string) []string {
	if content == "" {
		if note == "" {
			note = "waiting for the log…"
		}
		return []string{note}
	}
	return strings.Split(strings.TrimRight(content, "\n"), "\n")
}

// detailView draws the full-screen frame: header, the node's metrics
// unclipped to terminal width, a divider, the tailed log filling the
// remaining rows, a divider, and the footer naming the view's keys.
func (m dashModel) detailView() string {
	w := m.effWidth()
	e := m.entries[m.cursor]
	metricsLines, avail := m.detailSectionHeights()

	title := fmt.Sprintf("fleet dashboard  %s  node: %s", m.fleetPath, e.name)
	if e.node != nil {
		// A standing node never polls at all, follow flag or not, so its
		// header says nothing about a log state that can never change.
		logState := "following"
		if !m.detailLogFollow {
			logState = "paused"
		}
		title += "  log: " + logState
	}
	header := dashClip(title, w)
	divider := strings.Repeat("─", w)

	content := dashNodeContentLines(e.name, m.results[m.cursor], m.actions[m.cursor])
	for len(content) < metricsLines {
		content = append(content, "")
	}
	logLines := detailLogLines(m.detailLogContent, m.detailLogNote)
	if len(logLines) > avail {
		logLines = logLines[len(logLines)-avail:]
	}

	parts := make([]string, 0, len(content)+len(logLines)+4)
	parts = append(parts, header, divider)
	for _, line := range content {
		parts = append(parts, dashClip(line, w))
	}
	parts = append(parts, divider)
	for _, line := range logLines {
		parts = append(parts, dashClip(line, w))
	}
	parts = append(parts, divider)
	parts = append(parts, m.footerLine(w, dashFooterHints(dashDetailKeys, m.canAbort())))
	return strings.Join(parts, "\n")
}
