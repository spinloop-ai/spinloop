// A start's situation as data, and the one place it becomes text.
//
// A start that waits — for capacity, for a boot, for a dropped connection —
// has one situation at a time, and it holds for minutes. A caller that appends
// a line per transition to a scrolling log reads correctly whichever way that
// is carried; a caller that draws the most recent line in a fixed place (the
// dashboard tile) draws whatever the last write left there, including a
// situation the start has moved on from. A StartPhase is replaced outright on
// each transition and carries times rather than rendered numbers, so what is
// drawn is computed from the phase and the current time on every repaint.

package fleet

import (
	"fmt"
	"strings"
	"time"

	"github.com/spinloop-ai/spinloop/internal/remote"
)

// StartPhaseKind identifies what a start is currently doing. Exactly one value
// applies at a time.
type StartPhaseKind int

const (
	// PhaseAttempting: a request has been sent and no reply has come back.
	PhaseAttempting StartPhaseKind = iota
	// PhaseWaitingCapacity: the control plane refused the attempt for want of
	// capacity, and the next attempt is due at RetryAt.
	PhaseWaitingCapacity
	// PhaseBooting: the control plane reported the instance coming up.
	PhaseBooting
	// PhaseReconnecting: the attempt's connection dropped, and the next one is
	// due at RetryAt.
	PhaseReconnecting
)

// StartPhase is a start's current situation. It carries no rendered text and no
// rendered number: Since and RetryAt are the inputs RenderPhase computes one
// from, so a phase that holds for minutes still draws a value that moves.
type StartPhase struct {
	Kind StartPhaseKind
	// Since is when this phase began.
	Since time.Time
	// RetryAt is when the next attempt is due; zero outside a wait, and zero
	// during a wait whose reply named no delay.
	RetryAt time.Time
	// Detail is the control plane's state string, or a transport error.
	Detail string
}

// The two control-plane states the mapping reads by name. stateNoCapacity is
// an attempt refused because no zone had a GPU to give it — the one state that
// means the instance is not coming up at all. stateReady ends the start, and
// the caller reports its outcome rather than a phase.
const (
	stateNoCapacity = "no-capacity"
	stateReady      = "ready"
)

// RenderPhase is the phase's line at time now. A wait counts down towards
// RetryAt and a boot counts up from Since, so nothing here is fixed when the
// phase is built. The dashboard tile and `spinloop remote start` both draw
// their line from this, so the two cannot word one phase differently.
func RenderPhase(p StartPhase, now time.Time) string {
	switch p.Kind {
	case PhaseWaitingCapacity:
		return "waiting for capacity" + retryIn(p.RetryAt, now)
	case PhaseBooting:
		if p.Since.IsZero() {
			return "booting"
		}
		return "booting (" + formatPhaseDuration(now.Sub(p.Since)) + ")"
	case PhaseReconnecting:
		line := "connection dropped"
		if p.Detail != "" {
			line += " (" + p.Detail + ")"
		}
		return line + retryIn(p.RetryAt, now)
	default:
		return "waking the instance…"
	}
}

// retryIn is the tail a waiting phase carries: the time left until the next
// attempt is due, "" where the phase records no due time, and "retrying now"
// once the due time has passed — the moment between the wait ending and the
// next attempt reporting itself.
func retryIn(retryAt, now time.Time) string {
	if retryAt.IsZero() {
		return ""
	}
	left := retryAt.Sub(now)
	if left < time.Second {
		return " — retrying now"
	}
	return " — retrying in " + formatPhaseDuration(left)
}

// formatPhaseDuration renders a duration in whole seconds, in the shape the
// fleet surfaces already print an uptime in.
func formatPhaseDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	secs := int(d / time.Second)
	h, m, s := secs/3600, (secs/60)%60, secs%60
	switch {
	case h > 0:
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	case m > 0:
		return fmt.Sprintf("%dm %ds", m, s)
	default:
		return fmt.Sprintf("%ds", s)
	}
}

// StartPhases adapts remote.Start's progress and onState callbacks onto a
// stream of phases: it returns the pair to hand remote.Start, and calls report
// once per transition. The two callbacks are separate and neither carries a
// phase on its own — onState carries the state of a reply, and the progress
// line that follows a 503 carries that reply's retry-after — so the mapping
// holds the state between them.
//
// It is here rather than in either caller because both `spinloop remote start`
// and the dashboard drive remote.Start and render the result.
func StartPhases(report func(StartPhase)) (progress func(string), onState func(string)) {
	t := &startPhases{report: report, now: time.Now}
	return t.progress, t.state
}

// startPhases holds the current phase between the two callbacks.
type startPhases struct {
	report func(StartPhase)
	now    func() time.Time
	phase  StartPhase
	begun  bool
}

// set replaces the phase and reports it.
func (t *startPhases) set(p StartPhase) {
	t.phase = p
	t.begun = true
	t.report(p)
}

// enter moves to a phase kind, or leaves the current phase alone when it is
// already that kind — so Since keeps counting from when the situation began
// rather than restarting on every poll that reports the same thing.
func (t *startPhases) enter(kind StartPhaseKind, detail string) {
	if t.begun && t.phase.Kind == kind {
		return
	}
	t.set(StartPhase{Kind: kind, Since: t.now(), Detail: detail})
}

// state maps one reply's state onto a phase.
func (t *startPhases) state(s string) {
	switch {
	case s == remote.StateInFlight:
		// A fresh attempt supersedes a capacity wait and a dropped
		// connection: each described the attempt before it. It does not
		// supersede a boot — once a reply has reported the instance coming
		// up, the attempts that follow are polls of that same boot, and its
		// elapsed time counts from the first reply that reported it.
		if t.phase.Kind != PhaseBooting || !t.begun {
			t.enter(PhaseAttempting, "")
		}
	case s == stateReady:
		// The reply that ends the start. Its outcome is the caller's to
		// report; entering a phase here would draw a line for a situation
		// that is over before it can be read.
	case s == stateNoCapacity:
		// The reply's retry-after reaches this caller only on the progress
		// line remote.Start writes next, so the wait carries no due time
		// until that line arrives.
		t.enter(PhaseWaitingCapacity, s)
	default:
		t.enter(PhaseBooting, s)
	}
}

// droppedPrefix is how remote.Start opens the line it writes when an attempt's
// connection drops mid-request.
const droppedPrefix = "connection dropped"

// progress maps one of remote.Start's status lines onto a phase. The lines are
// the only place the 503's retry-after and a transport error reach this
// caller; which state a retry line refers to came through onState immediately
// before it, so only the delay is read off the line itself.
func (t *startPhases) progress(line string) {
	wait, hasWait := parseRetryIn(line)
	if strings.HasPrefix(line, droppedPrefix) {
		p := StartPhase{Kind: PhaseReconnecting, Since: t.now(), Detail: parenDetail(line)}
		if hasWait {
			p.RetryAt = p.Since.Add(wait)
		}
		t.set(p)
		return
	}
	if hasWait && t.phase.Kind == PhaseWaitingCapacity {
		t.phase.RetryAt = t.now().Add(wait)
		t.report(t.phase)
	}
}

// retryInMarker precedes the delay in every line remote.Start writes before a
// wait.
const retryInMarker = "retrying in "

// parseRetryIn reads the delay a status line names, in either of the forms
// remote.Start writes it — a whole number of seconds from the reply's
// retry-after, or a Go duration for the fixed wait after a dropped connection.
// A line naming no delay reports false, and the phase then carries no due time
// rather than a wrong one.
func parseRetryIn(line string) (time.Duration, bool) {
	i := strings.Index(line, retryInMarker)
	if i < 0 {
		return 0, false
	}
	fields := strings.Fields(line[i+len(retryInMarker):])
	if len(fields) == 0 {
		return 0, false
	}
	d, err := time.ParseDuration(strings.TrimRight(fields[0], "."))
	if err != nil || d < 0 {
		return 0, false
	}
	return d, true
}

// parenDetail is the parenthesised part of a status line — the transport error
// on a dropped connection — or "" when the line carries none.
func parenDetail(line string) string {
	open := strings.Index(line, "(")
	shut := strings.LastIndex(line, ")")
	if open < 0 || shut < open {
		return ""
	}
	return line[open+1 : shut]
}
