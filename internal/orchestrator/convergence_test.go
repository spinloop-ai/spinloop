package orchestrator

import (
	"context"
	"os"
	"testing"
	"time"
)

// workListWithLoop builds a work list over a real items file and an
// in-memory store, the dispatcher the recorder's: the loop's pieces under
// the test's hand, no run.
func workListWithLoop(t *testing.T, h *fakeHarness, rec *launchRecorder, content string) (*WorkList, string) {
	t.Helper()
	path := writeItems(t, content)
	store := newMemStore(t)
	d := NewDispatcher(h, "http://gateway:4000", "the-token", false)
	d.start = rec.start
	wl, err := NewWorkList(path, store, d, "http://gateway:4000", nil)
	if err != nil {
		t.Fatal(err)
	}
	return wl, path
}

// TestConsumeAborts_TheMarkerStopsTheItemAndHoldsThePass is the run's take-up
// of an abort marker, pass by pass: the agent stopped, the record and the
// marker gone, the item out of the pass that took it up, in the next.
func TestConsumeAborts_TheMarkerStopsTheItemAndHoldsThePass(t *testing.T) {
	topo := Topology{Wake: true, Nodes: []Node{runningNode("n", "org/m", nil)}}
	h := &fakeHarness{name: "opencode", bin: "unused"}
	factory, end := holdingFactory()
	rec := &launchRecorder{factory: factory}
	wl, path := workListWithLoop(t, h, rec, itemsFile(itemSpec{id: "a", dir: t.TempDir()}))

	wl.pass(topo, nil)
	if rec.launchCount() != 1 {
		t.Fatalf("the item is in flight before the ask: %d launches", rec.launchCount())
	}

	if err := RequestAbort(path, "a"); err != nil {
		t.Fatal(err)
	}
	suppressed := wl.consumeAborts()
	if !suppressed["a"] {
		t.Errorf("the pass that takes the marker up holds the item: %v", suppressed)
	}
	if ids, err := AbortsPending(path); err != nil || len(ids) != 0 {
		t.Errorf("the marker is taken up: %v (%v)", ids, err)
	}
	wl.mu.Lock()
	_, running := wl.inflight["a"]
	_, recorded := wl.records["a"]
	wl.mu.Unlock()
	if running || recorded {
		t.Errorf("the flight and the record are gone with the marker: running=%v recorded=%v", running, recorded)
	}
	wl.reap() // the stopped child's result stands nowhere: the slot is gone

	wl.pass(topo, suppressed)
	if rec.launchCount() != 1 {
		t.Errorf("the pass that took the marker up does not re-admit: %d launches", rec.launchCount())
	}
	wl.pass(topo, nil)
	if rec.launchCount() != 2 {
		t.Errorf("the next pass admits it again: %d launches", rec.launchCount())
	}
	end(1)
}

// TestPrune_AnInFlightItemTheFileLetGoKeepsItsRecord: the file lets go of an
// item that is running, the agent runs to its end, and the record goes with
// it out of the state once it is no longer in flight.
func TestPrune_AnInFlightItemTheFileLetGoKeepsItsRecord(t *testing.T) {
	topo := Topology{Wake: true, Nodes: []Node{runningNode("n", "org/m", nil)}}
	h := &fakeHarness{name: "opencode", bin: "unused"}
	factory, end := holdingFactory()
	rec := &launchRecorder{factory: factory}
	wl, path := workListWithLoop(t, h, rec, itemsFile(itemSpec{id: "a", dir: t.TempDir()}))

	wl.pass(topo, nil)
	if rec.launchCount() != 1 {
		t.Fatalf("the item is in flight before the file lets go: %d launches", rec.launchCount())
	}

	if err := os.WriteFile(path, []byte(itemsFile(itemSpec{id: "b", dir: t.TempDir()})), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := wl.refreshItems(); err != nil {
		t.Fatal(err)
	}
	wl.pass(topo, nil)
	wl.mu.Lock()
	st, recorded := wl.records["a"]
	wl.mu.Unlock()
	if !recorded || st.State != StateRunning {
		t.Errorf("the in-flight item's record stands until it ends: %+v (recorded %v)", st, recorded)
	}

	end(0)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		wl.reap()
		wl.mu.Lock()
		st, recorded = wl.records["a"]
		wl.mu.Unlock()
		if recorded && st.State == StateDone {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if !recorded || st.State != StateDone {
		t.Errorf("the item ran to its end, recorded done: %+v (recorded %v)", st, recorded)
	}

	wl.pass(topo, nil)
	wl.mu.Lock()
	_, recorded = wl.records["a"]
	wl.mu.Unlock()
	if recorded {
		t.Error("out of flight and out of the file, the record goes")
	}
}

// TestConsumeAborts_AMarkerForAnItemNotInFlight is taken up alone: the ask is
// consumed, and nothing else changes.
func TestConsumeAborts_AMarkerForAnItemNotInFlight(t *testing.T) {
	h := &fakeHarness{name: "opencode", bin: "unused"}
	rec := &launchRecorder{factory: func(bin string, args []string, dir, logPath string, env []string) (Child, error) {
		t.Fatal("nothing is launched")
		return nil, nil
	}}
	wl, path := workListWithLoop(t, h, rec, itemsFile(itemSpec{id: "a", dir: t.TempDir()}))

	wl.mu.Lock()
	wl.records["a"] = ItemState{State: StateDone, Node: "n", EndedAt: "2026-09-14T00:00:00Z"}
	wl.mu.Unlock()

	if err := RequestAbort(path, "a"); err != nil {
		t.Fatal(err)
	}
	suppressed := wl.consumeAborts()
	if suppressed["a"] {
		t.Errorf("a marker for an item not in flight takes up nothing to hold: %v", suppressed)
	}
	if ids, err := AbortsPending(path); err != nil || len(ids) != 0 {
		t.Errorf("the marker is taken up: %v (%v)", ids, err)
	}
	wl.mu.Lock()
	st, recorded := wl.records["a"]
	wl.mu.Unlock()
	if !recorded || st.State != StateDone {
		t.Errorf("nothing else changes: %+v (recorded %v)", st, recorded)
	}
}

// TestLoop_ANewPassTakesUpTheMarker works the whole hand-off: the item
// running, the command's marker beside the file, the run's pass taking it up,
// and the item back in the backlog where the next pass admits it.
func TestLoop_ANewPassTakesUpTheMarker(t *testing.T) {
	topo := &fakeTopo{}
	topo.set(Topology{Wake: true, Nodes: []Node{runningNode("n", "org/m", nil)}})
	h := &fakeHarness{name: "opencode", bin: "unused"}
	factory, _ := holdingFactory()
	rec := &launchRecorder{factory: factory}
	cfg, path := testConfig(t, topo, h, rec, itemsFile(itemSpec{id: "a", dir: t.TempDir()}))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Run(ctx, cfg) }()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if rec.launchCount() >= 1 && readState(t, path).Items["a"].State == StateRunning {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	if readState(t, path).Items["a"].State != StateRunning {
		cancel()
		t.Fatalf("the item is running before the ask (loop %v)", <-done)
	}
	if err := RequestAbort(path, "a"); err != nil {
		t.Fatal(err)
	}

	for time.Now().Before(deadline) {
		if rec.launchCount() >= 2 {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	if rec.launchCount() < 2 {
		cancel()
		t.Fatalf("the aborted item is admitted again on a later pass, got %d launches", rec.launchCount())
	}
	// The first agent was stopped, not run to its end: its record is gone,
	// the second launch is the item's re-admission, running in the state.
	if st := readState(t, path).Items["a"]; st.State != StateRunning {
		t.Errorf("the re-admitted item is running, got %+v", st)
	}
	if ids, err := AbortsPending(path); err != nil || len(ids) != 0 {
		t.Errorf("the marker is taken up: %v (%v)", ids, err)
	}
	cancel()
	if err := <-done; err != nil {
		t.Errorf("the loop should end on the cancel, not an error: %v", err)
	}
}

// TestLoop_ARemovedItemsRecordIsDropped is the file letting go of an item:
// the record of an item that is out of the file and out of flight goes, and
// the id that went out can come back in and be worked.
func TestLoop_ARemovedItemsRecordIsDropped(t *testing.T) {
	topo := &fakeTopo{}
	topo.set(Topology{Wake: true, Nodes: []Node{runningNode("n", "org/m", nil)}})
	h := &fakeHarness{name: "opencode", bin: "unused"}
	rec := &launchRecorder{factory: func(bin string, args []string, dir, logPath string, env []string) (Child, error) {
		return newFakeChild(nil, nil), nil
	}}
	cfg, path := testConfig(t, topo, h, rec, itemsFile(itemSpec{id: "a", dir: t.TempDir()}))

	err := loopRun(t, cfg, func(sf stateFile) bool {
		return sf.Items["a"].State == StateDone
	})
	if err != nil {
		t.Fatalf("the loop should end on the cancel, not an error: %v", err)
	}

	// The item out of the file: a run working it drops the record of it.
	if err := os.WriteFile(path, []byte(itemsFile(itemSpec{id: "b", dir: t.TempDir()})), 0o600); err != nil {
		t.Fatal(err)
	}
	err = loopRun(t, cfg, func(sf stateFile) bool {
		_, ok := sf.Items["a"]
		return !ok
	})
	if err != nil {
		t.Fatalf("the loop should end on the cancel, not an error: %v", err)
	}
	sf := readState(t, path)
	if _, ok := sf.Items["a"]; ok {
		t.Errorf("the record of the item the file let go stands in the state: %+v", sf.Items["a"])
	}

	// The id back in the file: no record to stand in its way, it is worked.
	if err := os.WriteFile(path, []byte(itemsFile(
		itemSpec{id: "b", dir: t.TempDir()},
		itemSpec{id: "a", dir: t.TempDir()},
	)), 0o600); err != nil {
		t.Fatal(err)
	}
	err = loopRun(t, cfg, func(sf stateFile) bool {
		return sf.Items["a"].State == StateDone
	})
	if err != nil {
		t.Fatalf("the loop should end on the cancel, not an error: %v", err)
	}
	if st := readState(t, path).Items["a"]; st.State != StateDone {
		t.Errorf("the id that went out comes back in and is worked, got %+v", st)
	}
}
