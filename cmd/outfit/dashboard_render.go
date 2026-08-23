// The `fleet dashboard` renderers: tiles, the grid, and the clipping that
// keeps a panel the shape of the frame. A panel draws the very lines the bar
// format of `fleet metrics` prints for the same node — renderStatBars,
// renderTokenLines and the last-active line are shared, not reimplemented —
// so the two surfaces cannot word a number differently.
//
// Lipgloss frames the panel; everything it receives is already exactly
// dashTileW wide and dashTileH tall, so the frame never has to decide how to
// fit anything — it only pads and borders. That is what keeps the grid
// rectangular on every terminal size.

package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
	ansi "github.com/charmbracelet/x/ansi"
	"github.com/lucinate-ai/outfit/internal/fleet"
	"github.com/lucinate-ai/outfit/internal/metrics"
)

// Tile geometry. Content is dashTileW by dashTileH; the frame adds one
// character per side, and neighbours sit one space apart. The bar line is the
// width constraint: 41 columns, so 42 fits it whole; a running node with one
// GPU fills all 12 lines exactly.
const (
	dashTileW    = 42
	dashTileH    = 12
	dashTileStep = dashTileW + 2 + 1 // frame plus the gap to the next tile
	dashTileRowH = dashTileH + 2 + 1
)

// The frame's fixed rows above and below the grid.
const (
	dashHeaderH = 1
	dashFooterH = 1
)

// dashCols is how many tiles fit across at a given width.
func dashCols(w int) int {
	c := (w + 1) / dashTileStep
	if c < 1 {
		return 1
	}
	return c
}

// dashVisibleRows is how many tile rows sit between header and footer at a
// given height.
func dashVisibleRows(h int) int {
	r := (h - dashHeaderH - dashFooterH) / dashTileRowH
	if r < 1 {
		return 1
	}
	return r
}

// dashClampScroll bounds the first visible row so the grid never scrolls
// past its ends.
func dashClampScroll(scrollRow, entries, w, h int) int {
	cols := dashCols(w)
	rows := (entries + cols - 1) / cols
	if rows < 1 {
		rows = 1
	}
	top := rows - dashVisibleRows(h)
	if top < 0 {
		top = 0
	}
	if scrollRow < 0 {
		return 0
	}
	if scrollRow > top {
		return top
	}
	return scrollRow
}

// dashClip hard-cuts a line at width display columns. It is a cut, not a wrap
// — wrapping would push a bar line onto two rows and break the block's
// alignment — and it is ANSI-aware (ansi.CutWc), so a colour code mid-line
// survives and a wide character counts as two.
func dashClip(line string, width int) string {
	if lipgloss.Width(line) <= width {
		return line
	}
	return ansi.CutWc(line, 0, width)
}

// dashTileContent is one panel's inside: the node's name and the facts the
// bar format prints for it. The shapes are the state of the node's business
// — a start or stop in flight (the call's own status lines beside the node's
// last report: the call says what the operator asked for, the report says
// what the node is doing — a boot half done already carries a state and
// whatever it measures, and that is worth seeing while the call works), not
// answered yet (an empty panel naming the node), answered and working (the
// full bar block), or answered and not working (outcome plus reason, which
// is what every one-shot surface shows).
func dashTileContent(name string, r fleet.NodeResult, a dashAction) string {
	var b strings.Builder
	switch {
	case a.verb != "":
		fmt.Fprintf(&b, "%s  %s\n", name, dashVerbProgress(a.verb))
		if a.line != "" {
			fmt.Fprintln(&b, a.line)
		}
		if r.OK() {
			if s := r.Metrics.State; s != "" {
				fmt.Fprintln(&b, dashStateLine(s, r.Metrics))
			}
			dashTileReportBody(&b, r.Metrics, true)
		}
	case r.Outcome == "":
		fmt.Fprintf(&b, "%s\nwaiting for first refresh…\n", name)
	case !r.OK():
		fmt.Fprintf(&b, "%s  %s\n", name, r.Outcome)
		if d := r.Detail(); d != "" {
			fmt.Fprintln(&b, d)
		}
	default:
		fmt.Fprintf(&b, "%s  %s\n", name, dashStateLine(r.Metrics.State, r.Metrics))
		dashTileReportBody(&b, r.Metrics, r.Metrics.State == "running")
	}
	lines := strings.Split(b.String(), "\n")
	lines = lines[:len(lines)-1] // the trailing newline splits an extra empty piece
	for len(lines) < dashTileH {
		lines = append(lines, "")
	}
	if len(lines) > dashTileH {
		lines = lines[:dashTileH]
	}
	for i, line := range lines {
		lines[i] = dashClip(line, dashTileW)
	}
	return strings.Join(lines, "\n")
}

// dashTileServingLine is a report's "what it serves" line: the runner and
// the model — or "" when the answer carries neither. Uptime rides on the
// tile's state line instead (dashStateLine), where a long runner or model ID
// cannot clip it off.
func dashTileServingLine(m metrics.Stats) string {
	line := m.Runner
	if m.ModelID != "" {
		if line != "" {
			line += "  "
		}
		line += m.ModelID
	}
	return line
}

// dashStateLine pairs a node's state with its uptime, when it has one, so
// uptime stays visible on the tile's state line rather than risking the
// clip on the (potentially long) serving line below it.
func dashStateLine(state string, m metrics.Stats) string {
	if m.UptimeSeconds <= 0 {
		return state
	}
	uptime := "(up " + formatDuration(m.UptimeSeconds) + ")"
	if state == "" {
		return uptime
	}
	return state + "  " + uptime
}

// dashTileReportBody appends what a node's last answer carries beneath its
// name line: the serving line, the last-active line, and — when resources is
// set — the resource bars and token counters. Each part prints only when the
// answer has it. A settled tile gates the resources block on the node being
// running; the in-flight tile draws whatever there is, because a boot half
// done has some of the facts and not the rest.
func dashTileReportBody(w io.Writer, m metrics.Stats, resources bool) {
	if line := dashTileServingLine(m); line != "" {
		fmt.Fprintln(w, line)
	}
	renderLastActiveIndented(w, m.LastActiveAt, m.IdleSeconds)
	if resources {
		renderStatBars(w, m.CPU, m.Memory, m.GPUs)
		renderTokenLines(w, m.Tokens)
	}
}

// dashTile frames one panel; the selected one carries a lit border.
func dashTile(name string, r fleet.NodeResult, selected bool, a dashAction) string {
	style := lipgloss.NewStyle().
		Width(dashTileW).Height(dashTileH).
		Border(lipgloss.RoundedBorder())
	if selected {
		style = style.BorderForeground(lipgloss.Color("214"))
	} else {
		style = style.BorderForeground(lipgloss.Color("240"))
	}
	return style.Render(dashTileContent(name, r, a))
}

// dashGridRows lays tiles out left to right, top to bottom, in fleet-file
// order — the same order every one-shot surface lists the nodes in. A grid
// row joins the corresponding lines of the tiles it places, a missing line
// as an empty cell. Joining the tiles themselves would do that once, between
// the blocks: each tile is a dashTileH+2-line block, so the second tile's
// top border would land beside the first tile's bottom border and its body
// would shift down a line.
func dashGridRows(tiles []string, cols int) []string {
	lines := make([][]string, len(tiles))
	for i, tile := range tiles {
		lines[i] = strings.Split(tile, "\n")
	}
	rows := make([]string, 0, (len(tiles)+cols-1)/cols)
	for i := 0; i < len(tiles); i += cols {
		end := i + cols
		if end > len(tiles) {
			end = len(tiles)
		}
		var body []string
		for line := 0; ; line++ {
			cells := make([]string, end-i)
			done := true
			for c := i; c < end; c++ {
				if line < len(lines[c]) {
					cells[c-i] = lines[c][line]
					done = false
				}
			}
			if done {
				break
			}
			body = append(body, strings.Join(cells, " "))
		}
		rows = append(rows, strings.Join(body, "\n"))
	}
	return rows
}
