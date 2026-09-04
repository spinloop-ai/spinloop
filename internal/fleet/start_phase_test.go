package fleet

import (
	"strings"
	"testing"
	"time"

	"github.com/spinloop-ai/spinloop/internal/remote"
)

// Every line a phase renders is computed from the phase and the time it is
// drawn at, so a phase that holds while the clock moves draws a different line
// each time. The clock here is fixed and moved by hand: nothing in a phase is
// read from the wall clock, so the same phase at two times is the whole test.
func TestRenderPhaseIsComputedAtDrawTime(t *testing.T) {
	base := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)

	// A capacity wait counts down towards the attempt it is waiting for.
	wait := StartPhase{Kind: PhaseWaitingCapacity, Since: base, RetryAt: base.Add(120 * time.Second), Detail: "no-capacity"}
	if got, want := RenderPhase(wait, base), "waiting for capacity — retrying in 2m 0s"; got != want {
		t.Errorf("RenderPhase(wait, base) = %q, want %q", got, want)
	}
	if got, want := RenderPhase(wait, base.Add(73*time.Second)), "waiting for capacity — retrying in 47s"; got != want {
		t.Errorf("the wait did not count down: %q, want %q", got, want)
	}
	// Past its due time, with the next attempt not yet reported.
	if got, want := RenderPhase(wait, base.Add(130*time.Second)), "waiting for capacity — retrying now"; got != want {
		t.Errorf("RenderPhase past the due time = %q, want %q", got, want)
	}
	// A refusal whose reply named no delay says what it is waiting for and
	// nothing about when, rather than a wrong number.
	noDue := StartPhase{Kind: PhaseWaitingCapacity, Since: base, Detail: "no-capacity"}
	if got, want := RenderPhase(noDue, base.Add(time.Minute)), "waiting for capacity"; got != want {
		t.Errorf("RenderPhase with no due time = %q, want %q", got, want)
	}

	// A boot counts up from when it began.
	boot := StartPhase{Kind: PhaseBooting, Since: base, Detail: "starting"}
	if got, want := RenderPhase(boot, base.Add(4*time.Minute+12*time.Second)), "booting (4m 12s)"; got != want {
		t.Errorf("RenderPhase(boot) = %q, want %q", got, want)
	}
	if got, want := RenderPhase(boot, base.Add(2*time.Hour)), "booting (2h 0m 0s)"; got != want {
		t.Errorf("the boot did not count up: %q, want %q", got, want)
	}

	// An attempt in flight, and a dropped connection carrying the transport
	// error it dropped with.
	if got, want := RenderPhase(StartPhase{Kind: PhaseAttempting, Since: base}, base), "waking the instance…"; got != want {
		t.Errorf("RenderPhase(attempting) = %q, want %q", got, want)
	}
	dropped := StartPhase{Kind: PhaseReconnecting, Since: base, RetryAt: base.Add(5 * time.Second), Detail: "EOF"}
	if got, want := RenderPhase(dropped, base.Add(time.Second)), "connection dropped (EOF) — retrying in 4s"; got != want {
		t.Errorf("RenderPhase(reconnecting) = %q, want %q", got, want)
	}
}

// The mapping from remote.Start's two callbacks onto phases: an attempt goes
// out, is refused for capacity with a due time for the next one, and the
// attempt that follows retires the refusal rather than leaving it standing
// while the instance boots — the defect this phase stream exists for.
func TestStartPhasesRetiresACapacityWait(t *testing.T) {
	var got []StartPhase
	progress, onState := StartPhases(func(p StartPhase) { got = append(got, p) })

	onState(remote.StateInFlight)
	onState("no-capacity")
	progress("instance no-capacity; retrying in 120s")
	onState(remote.StateInFlight)

	kinds := make([]StartPhaseKind, len(got))
	for i, p := range got {
		kinds[i] = p.Kind
	}
	want := []StartPhaseKind{PhaseAttempting, PhaseWaitingCapacity, PhaseWaitingCapacity, PhaseAttempting}
	if len(kinds) != len(want) {
		t.Fatalf("phases = %v, want %v", kinds, want)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("phases = %v, want %v", kinds, want)
		}
	}
	// The retry-after reaches this caller only on the progress line, and it
	// lands on the wait the state established.
	due := got[2].RetryAt.Sub(got[2].Since)
	if due < 119*time.Second || due > 121*time.Second {
		t.Errorf("the capacity wait's due time is %s after it began, want about 2m", due)
	}
	if last := got[len(got)-1]; last.Kind == PhaseWaitingCapacity {
		t.Error("the capacity wait outlived the attempt that superseded it")
	}
}

// A reply reporting the instance coming up enters the boot, and the polls that
// follow are that same boot: its elapsed time counts from the first reply that
// reported it, rather than restarting on each poll.
func TestStartPhasesHoldsTheBootAcrossPolls(t *testing.T) {
	var got []StartPhase
	progress, onState := StartPhases(func(p StartPhase) { got = append(got, p) })

	onState(remote.StateInFlight)
	onState("starting")
	progress("instance starting; retrying in 5s")
	onState(remote.StateInFlight)
	onState("starting")

	if len(got) != 2 {
		t.Fatalf("phases = %+v, want the attempt and the boot only", got)
	}
	if got[1].Kind != PhaseBooting || got[1].Detail != "starting" {
		t.Errorf("second phase = %+v, want a boot reporting the control plane's state", got[1])
	}
}

// A dropped connection is its own phase, carrying the transport error and the
// wait before the retry, and the attempt that follows retires it.
func TestStartPhasesReportsADroppedConnection(t *testing.T) {
	var got []StartPhase
	progress, onState := StartPhases(func(p StartPhase) { got = append(got, p) })

	onState(remote.StateInFlight)
	progress("connection dropped (unexpected EOF); retrying in 5s")

	last := got[len(got)-1]
	if last.Kind != PhaseReconnecting {
		t.Fatalf("last phase = %+v, want a reconnect", last)
	}
	if last.Detail != "unexpected EOF" {
		t.Errorf("detail = %q, want the transport error", last.Detail)
	}
	if line := RenderPhase(last, last.Since.Add(time.Second)); !strings.Contains(line, "retrying in 4s") {
		t.Errorf("rendered = %q, want the wait counting down", line)
	}
}
