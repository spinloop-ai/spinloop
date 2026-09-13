package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/spinloop-ai/spinloop/internal/catalog"
	"github.com/spinloop-ai/spinloop/internal/harness"
	"github.com/spinloop-ai/spinloop/internal/spinloop"
)

// --- fixtures ---------------------------------------------------------------

// itemSpec is one work item for building a file.
type itemSpec struct {
	id    string
	dir   string
	instr string
	// extra are additional field lines, already indented two spaces.
	extra []string
}

func (s itemSpec) render() string {
	if s.instr == "" {
		s.instr = "do"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "- id: %s\n  instructions: %s\n  dir: %s", s.id, s.instr, s.dir)
	for _, line := range s.extra {
		b.WriteString("\n" + line)
	}
	b.WriteString("\n")
	return b.String()
}

// itemsFile renders a work items file: a top-level list of items.
func itemsFile(specs ...itemSpec) string {
	var b strings.Builder
	for _, s := range specs {
		b.WriteString(s.render())
	}
	return b.String()
}

// writeItems writes the items file to a fresh directory and returns its path.
func writeItems(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "work.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// readState reads the state beside an items file.
func readState(t *testing.T, itemsPath string) stateFile {
	t.Helper()
	data, err := os.ReadFile(itemsPath + ".state.json")
	if err != nil {
		return stateFile{Items: map[string]ItemState{}}
	}
	var sf stateFile
	if err := json.Unmarshal(data, &sf); err != nil {
		t.Fatalf("the state is not a record: %v", err)
	}
	return sf
}

// fakeTopo is a Topologist whose reply the test holds.
type fakeTopo struct {
	mu   sync.Mutex
	topo Topology
	fail bool
	err  error
}

func (f *fakeTopo) set(topo Topology) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.topo = topo
	f.fail = false
	f.err = nil
}

func (f *fakeTopo) failWith(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fail = true
	f.err = err
}

func (f *fakeTopo) Topology(context.Context) (Topology, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail {
		return Topology{}, f.err
	}
	return f.topo, nil
}

// appliedRecord is one apply the fake harness was asked to make.
type appliedRecord struct {
	sel        spinloop.Selection
	setDefault bool
	keyEnv     string // what the resolve answered for the plumbing's key variable
}

// fakeHarness is a harness that records its applies and launches nothing:
// Name and Command are what the dispatcher asks, everything else stands in.
type fakeHarness struct {
	mu      sync.Mutex
	name    string
	bin     string
	applied []appliedRecord
}

func (h *fakeHarness) Name() string                { return h.name }
func (h *fakeHarness) Command() string             { return h.bin }
func (h *fakeHarness) ConfigPath() (string, error) { return "fake-config", nil }

func (h *fakeHarness) Apply(p *catalog.Provider, sel spinloop.Selection, contextWindow, outputTokens int, setDefaultModel bool, resolve func(string) string) (harness.Summary, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.applied = append(h.applied, appliedRecord{sel: sel, setDefault: setDefaultModel, keyEnv: resolve(p.APIKeyEnv)})
	return harness.Summary{}, nil
}

func (h *fakeHarness) Remove(providerID string, modelKeys []string) (int, error) {
	return 0, nil
}

func (h *fakeHarness) State() (map[string]harness.ProviderState, string, error) {
	return nil, "", nil
}

func (h *fakeHarness) applyCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.applied)
}

// fakeChild is a launched agent the test controls: Wait blocks on its
// release, Stop and Kill record themselves and end the wait.
type fakeChild struct {
	mu      sync.Mutex
	err     error
	release chan struct{}
	stopped bool
	killed  bool
}

func newFakeChild(err error, release chan struct{}) *fakeChild {
	return &fakeChild{err: err, release: release}
}

func (c *fakeChild) Wait() error {
	if c.release != nil {
		<-c.release
	}
	return c.err
}

func (c *fakeChild) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.stopped {
		c.stopped = true
		if c.release != nil {
			close(c.release)
		}
	}
}

func (c *fakeChild) Kill() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.killed {
		c.killed = true
		if c.release != nil {
			close(c.release)
		}
	}
}

func (c *fakeChild) wasStopped() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stopped
}

// launchRecorder is the start seam the loop tests use: it records every
// launch and its in-flight count, and hands back the child the factory makes.
type launchRecorder struct {
	mu       sync.Mutex
	launched []launch
	inFlight int
	max      int
	factory  func(bin string, args []string, dir, logPath string, env []string) (Child, error)
}

func (r *launchRecorder) start(bin string, args []string, dir, logPath string, env []string) (Child, error) {
	child, err := r.factory(bin, args, dir, logPath, env)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	r.launched = append(r.launched, launch{bin: bin, dir: dir, args: args})
	r.inFlight++
	if r.inFlight > r.max {
		r.max = r.inFlight
	}
	r.mu.Unlock()
	go func() {
		child.Wait()
		r.mu.Lock()
		r.inFlight--
		r.mu.Unlock()
	}()
	return child, nil
}

// launch is one launch's identity, as recorded.
type launch struct {
	bin  string
	dir  string
	args []string
}

func (r *launchRecorder) launchCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.launched)
}

func (r *launchRecorder) launchArgs() [][]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([][]string, len(r.launched))
	for i, l := range r.launched {
		out[i] = l.args
	}
	return out
}

func (r *launchRecorder) inFlightCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.inFlight
}

func (r *launchRecorder) maxInFlight() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.max
}

// holdingFactory returns a start seam whose children each hold on their own
// release — so ending one ends exactly one — and a function that ends the
// child the factory made nth.
func holdingFactory() (
	func(bin string, args []string, dir, logPath string, env []string) (Child, error),
	func(n int),
) {
	var mu sync.Mutex
	var holds []chan struct{}
	factory := func(bin string, args []string, dir, logPath string, env []string) (Child, error) {
		release := make(chan struct{})
		mu.Lock()
		holds = append(holds, release)
		mu.Unlock()
		return newFakeChild(nil, release), nil
	}
	end := func(n int) {
		mu.Lock()
		defer mu.Unlock()
		close(holds[n])
	}
	return factory, end
}

// runningNode is a node the gateway reports running the named model.
func runningNode(name, model string, tags map[string]string) Node {
	return Node{Name: name, Kind: "daemon", Tags: tags, State: "running", Model: model, Ready: "ready"}
}

// stoppedNode is a node the gateway reports stopped; a request would start it
// with the named model.
func stoppedNode(name, wakeable string, tags map[string]string) Node {
	return Node{Name: name, Kind: "daemon", Tags: tags, State: "stopped", WakeableModel: wakeable}
}

// --- 3.1: the work items file ------------------------------------------------

func TestParseItems_MinimalItem(t *testing.T) {
	items, err := ParseItems([]byte(itemsFile(itemSpec{id: "a", instr: "do a", dir: "./a"})))
	if err != nil {
		t.Fatalf("a minimal item should parse: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("one item should parse, got %d", len(items))
	}
	it := items[0]
	if it.ID != "a" || it.Instructions != "do a" || it.Dir != "./a" {
		t.Errorf("the item's fields should be the file's, got %+v", it)
	}
	if len(it.Tags) != 0 || it.Priority != 0 {
		t.Errorf("a minimal item matches any node, at the lowest priority, got %+v", it)
	}
}

func TestParseItems_DuplicateIDFailsNamingTheID(t *testing.T) {
	_, err := ParseItems([]byte(itemsFile(
		itemSpec{id: "a", instr: "one", dir: "./a"},
		itemSpec{id: "a", instr: "two", dir: "./b"},
	)))
	if err == nil {
		t.Fatal("a duplicate id should be refused")
	}
	if !strings.Contains(err.Error(), `id "a"`) {
		t.Errorf("the refusal should name the duplicated id, got %v", err)
	}
}

func TestParseItems_MissingFieldsNameTheItem(t *testing.T) {
	_, err := ParseItems([]byte("- id: a\n  dir: ./a\n"))
	if err == nil || !strings.Contains(err.Error(), `item "a"`) || !strings.Contains(err.Error(), "instructions") {
		t.Errorf("a missing instructions should fail naming the item, got %v", err)
	}
	_, err = ParseItems([]byte("- id: a\n  instructions: do a\n"))
	if err == nil || !strings.Contains(err.Error(), `item "a"`) || !strings.Contains(err.Error(), "working directory") {
		t.Errorf("a missing working directory should fail naming the item, got %v", err)
	}
}

func TestParseItems_UnparseableFileFails(t *testing.T) {
	if _, err := ParseItems([]byte("not: [a list\n")); err == nil {
		t.Error("a file that is not a list of items should fail")
	}
	if _, err := LoadItems("/nonexistent/work.yaml"); err == nil {
		t.Error("an unreadable file should fail")
	}
}

func TestParseItems_Tags(t *testing.T) {
	items, err := ParseItems([]byte(itemsFile(itemSpec{
		id: "a", dir: "./a",
		extra: []string{"  tags:", "    - gpu=a100", "    - os=linux"},
	})))
	if err != nil {
		t.Fatalf("tagged items should parse: %v", err)
	}
	if len(items[0].Tags) != 2 || items[0].Tags[0] != "gpu=a100" || items[0].Tags[1] != "os=linux" {
		t.Errorf("the tags should ride on the item, got %v", items[0].Tags)
	}
	_, err = ParseItems([]byte(itemsFile(itemSpec{
		id: "a", dir: "./a", extra: []string{"  tags:", "    - notatag"},
	})))
	if err == nil || !strings.Contains(err.Error(), `item "a"`) || !strings.Contains(err.Error(), `notatag`) {
		t.Errorf("a malformed tag should fail naming the item and the tag, got %v", err)
	}
	_, err = ParseItems([]byte(itemsFile(itemSpec{
		id: "a", dir: "./a", extra: []string{"  tags:", "    - gpu=a100", "    - gpu=h100"},
	})))
	if err == nil || !strings.Contains(err.Error(), `item "a"`) {
		t.Errorf("a repeated tag key should fail naming the item, got %v", err)
	}
}

func TestLoadItems_PriorityAndFileOrder(t *testing.T) {
	items, err := LoadItems(writeItems(t, itemsFile(
		itemSpec{id: "low", dir: "./low"},
		itemSpec{id: "high", dir: "./high", extra: []string{"  priority: 10"}},
		itemSpec{id: "mid", dir: "./mid", extra: []string{"  priority: 1"}},
	)))
	if err != nil {
		t.Fatal(err)
	}
	if items[0].Priority != 0 || items[1].Priority != 10 || items[2].Priority != 1 {
		t.Errorf("the priorities should be the file's, got %+v", items)
	}
	sorted := sortedForAdmission(items)
	want := []string{"high", "mid", "low"}
	for i, id := range want {
		if sorted[i].ID != id {
			t.Fatalf("admission order should be priority then file order, got %v", sorted)
		}
	}
}

func TestLoadItems_ZeroPriorityIsStated(t *testing.T) {
	items, err := LoadItems(writeItems(t, itemsFile(itemSpec{
		id: "a", dir: "./a", extra: []string{"  priority: 0"},
	})))
	if err != nil {
		t.Fatal(err)
	}
	if items[0].Priority != 0 {
		t.Errorf("a stated zero is the lowest rank, got %d", items[0].Priority)
	}
}

// --- 3.2: matching and admission --------------------------------------------

func TestMatch_NoTagsMatchesAny(t *testing.T) {
	item := Item{ID: "a", Tags: nil}
	topo := Topology{Wake: true, Nodes: []Node{
		runningNode("run", "org/m", nil),
		stoppedNode("stop", "org/s", nil),
		{Name: "dead", Kind: "daemon", State: "unreachable", Detail: "no route to host"},
	}}
	got := Match(item, topo)
	if len(got) != 2 || got[0].Name != "run" || got[1].Name != "stop" {
		t.Errorf("an untagged item matches any answered node, running first, got %+v", got)
	}
}

func TestMatch_TagsMustAllBeCarried(t *testing.T) {
	// The spec's scenario: two nodes each carry one of the item's two tags,
	// a third carries both — the item matches only the third.
	item := Item{ID: "a", Tags: []string{"gpu=a100", "os=linux"}}
	topo := Topology{Wake: true, Nodes: []Node{
		runningNode("gpu-only", "org/m", map[string]string{"gpu": "a100"}),
		runningNode("os-only", "org/m", map[string]string{"os": "linux"}),
		runningNode("both", "org/m", map[string]string{"gpu": "a100", "os": "linux"}),
	}}
	got := Match(item, topo)
	if len(got) != 1 || got[0].Name != "both" {
		t.Errorf("every tag the item carries must be the node's, got %+v", got)
	}
}

func TestMatch_RunningOfferedBeforeStopped(t *testing.T) {
	item := Item{ID: "a"}
	topo := Topology{Wake: true, Nodes: []Node{
		stoppedNode("stop", "org/s", nil),
		runningNode("run", "org/m", nil),
	}}
	got := Match(item, topo)
	if len(got) != 2 || got[0].Name != "run" || got[1].Name != "stop" {
		t.Errorf("a running node is offered before one to be started, got %+v", got)
	}
}

func TestMatch_StoppedOnlyWhereTheFleetWakes(t *testing.T) {
	item := Item{ID: "a"}
	topo := Topology{Wake: false, Nodes: []Node{
		stoppedNode("stop", "org/s", nil),
	}}
	if got := Match(item, topo); len(got) != 0 {
		t.Errorf("a stopped node is no candidate where the fleet does not wake, got %+v", got)
	}
}

func TestMatch_DeadNodeIsNoCandidate(t *testing.T) {
	item := Item{ID: "a"}
	topo := Topology{Wake: true, Nodes: []Node{
		{Name: "dead", Kind: "daemon", Tags: map[string]string{}, State: "unreachable", Detail: "no route to host"},
	}}
	if got := Match(item, topo); len(got) != 0 {
		t.Errorf("a node the gateway did not reach is no candidate, got %+v", got)
	}
}

func TestMatch_RankedByTheFleetsPreference(t *testing.T) {
	item := Item{ID: "a"}
	nodes := []Node{
		runningNode("recent", "org/m", nil),
		runningNode("stale", "org/m", nil),
		runningNode("never", "org/m", nil),
	}
	nodes[0].LastActiveAt = "2026-01-02T00:00:00Z"
	nodes[1].LastActiveAt = "2026-01-01T00:00:00Z"

	// Idle: the node inactive longest comes first, a never-active node most
	// idle of all.
	idle := Match(item, Topology{Wake: true, Nodes: nodes})
	if idle[0].Name != "never" || idle[1].Name != "stale" || idle[2].Name != "recent" {
		t.Errorf("idle ranks the inactive longest first, got %+v", idle)
	}
	// Active: the most recently active comes first, a never-active node last.
	topo := Topology{Wake: true, Prefer: "active", Nodes: nodes}
	active := Match(item, topo)
	if active[0].Name != "recent" || active[1].Name != "stale" || active[2].Name != "never" {
		t.Errorf("active ranks the most recently active first, got %+v", active)
	}
}

func TestMatch_NamesTheWakeableModel(t *testing.T) {
	item := Item{ID: "a"}
	topo := Topology{Wake: true, Nodes: []Node{
		stoppedNode("stop", "org/served", nil),
	}}
	got := Match(item, topo)
	if len(got) != 1 || got[0].ModelName() != "org/served" {
		t.Errorf("a stopped node is offered the model a request would start it with, got %+v", got)
	}
	run := runningNode("run", "org/id", nil)
	run.ServedName = "the-alias"
	if run.ModelName() != "the-alias" {
		t.Errorf("a running node is offered its served name, got %q", run.ModelName())
	}
	run.ServedName = ""
	if run.ModelName() != "org/id" {
		t.Errorf("without a served name the model id stands, got %q", run.ModelName())
	}
}

func total(n int) *int { return &n }

func TestAdmits_TotalBoundsTheInFlight(t *testing.T) {
	topo := Topology{Concurrency: &Concurrency{Total: total(3)}}
	three := []Item{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	if Admits(topo, three, Item{ID: "d"}) {
		t.Error("at the total, no further item is admitted, whatever its tags")
	}
	if !Admits(topo, three[:2], Item{ID: "d"}) {
		t.Error("below the total, an item is admitted")
	}
}

func TestAdmits_TagLimitBoundsItsOwnItems(t *testing.T) {
	topo := Topology{Concurrency: &Concurrency{
		Total: total(10),
		Tags:  map[string]int{"gpu=a100": 2},
	}}
	carrying := []Item{
		{ID: "a", Tags: []string{"gpu=a100"}},
		{ID: "b", Tags: []string{"gpu=a100"}},
	}
	if Admits(topo, carrying, Item{ID: "c", Tags: []string{"gpu=a100"}}) {
		t.Error("at its tag limit, a further item carrying the tag waits")
	}
	if !Admits(topo, carrying, Item{ID: "d", Tags: []string{"cpu=big"}}) {
		t.Error("an item carrying no such tag may still be admitted")
	}
	if !Admits(topo, carrying, Item{ID: "e"}) {
		t.Error("an untagged item counts against no tag limit")
	}
}

func TestAdmits_AnItemCountsAgainstEveryTagItCarries(t *testing.T) {
	topo := Topology{Concurrency: &Concurrency{Tags: map[string]int{"gpu=a100": 1, "os=linux": 1}}}
	inflight := []Item{{ID: "a", Tags: []string{"gpu=a100", "os=linux"}}}
	if Admits(topo, inflight, Item{ID: "b", Tags: []string{"os=linux"}}) {
		t.Error("an in-flight item counts against every tag it carries")
	}
}

func TestAdmits_NoLimitsNoBounding(t *testing.T) {
	topo := Topology{}
	many := make([]Item, 50)
	for i := range many {
		many[i] = Item{ID: fmt.Sprintf("a%d", i)}
	}
	if !Admits(topo, many, Item{ID: "next"}) {
		t.Error("where the fleet declares no limits, every item a node will take is admitted")
	}
}

// --- 3.3: state beside the items file ---------------------------------------

func writeState(t *testing.T, itemsPath string, sf stateFile) {
	t.Helper()
	data, err := json.Marshal(sf)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(itemsPath+".state.json", data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestStore_RestartRecordsARunningItemFailed(t *testing.T) {
	path := writeItems(t, itemsFile(itemSpec{id: "a", dir: "./a"}))
	writeState(t, path, stateFile{Items: map[string]ItemState{
		"a":  {State: StateRunning, Node: "n1"},
		"ok": {State: StateDone, Node: "n1"},
		"no": {State: StateFailed, Why: "the agent ended in error"},
	}})

	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	sf := readState(t, path)
	if sf.Items["a"].State != StateFailed {
		t.Errorf("an item left running is recorded failed, got %+v", sf.Items["a"])
	}
	if !strings.Contains(sf.Items["a"].Why, "stopped while it was running") {
		t.Errorf("the record should name the interruption, got %q", sf.Items["a"].Why)
	}
	if sf.Items["ok"].State != StateDone || sf.Items["no"].State != StateFailed {
		t.Errorf("a recorded end stands across a restart, got %+v", sf.Items)
	}
}

func TestStore_ASecondOrchestratorForTheSameFileIsRefused(t *testing.T) {
	path := writeItems(t, itemsFile(itemSpec{id: "a", dir: "./a"}))
	first, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	if _, err := OpenStore(path); err == nil {
		t.Fatal("a second orchestrator for the same items file should be refused")
	} else if !strings.Contains(err.Error(), fmt.Sprintf("pid %d", os.Getpid())) {
		t.Errorf("the refusal should name the first, got %v", err)
	}
}

func TestStore_AStaleLockIsTakenOver(t *testing.T) {
	// A process that has already exited, so its lock is stale.
	cmd := exec.Command("true")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	dead := cmd.Process.Pid
	cmd.Wait()

	path := writeItems(t, itemsFile(itemSpec{id: "a", dir: "./a"}))
	if err := os.WriteFile(path+".lock", []byte(fmt.Sprintf("%d", dead)), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(path)
	if err != nil {
		t.Fatalf("a lock whose process is gone is taken over, not fatal: %v", err)
	}
	store.Close()
	if _, err := os.Stat(path + ".lock"); !os.IsNotExist(err) {
		t.Error("a closed store leaves no lock")
	}
}

// --- 3.4: dispatch ------------------------------------------------------------

// stubAgent is the stub harness executable: it records where it was told to
// work, what it was told, and what it was given, then says its piece.
func stubAgent(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "agent")
	script := `#!/bin/sh
{
  echo "cwd:$(pwd)"
  for a in "$@"; do echo "arg:$a"; done
  echo "key:$OPENAI_API_KEY"
} > "$RECORD_FILE"
echo "the agent's output"
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func readRecord(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the stub wrote no record: %v", err)
	}
	return string(data)
}

func TestDispatch_RunsTheAgentOneShotInTheItemsDirectory(t *testing.T) {
	work := t.TempDir()
	record := filepath.Join(work, "record")
	t.Setenv("RECORD_FILE", record)
	t.Setenv("OPENAI_API_KEY", "")
	bin := stubAgent(t)

	h := &fakeHarness{name: "opencode", bin: bin}
	d := NewDispatcher(h, "http://gateway:4000", "the-token")
	child, err := d.Launch(
		Item{ID: "a", Instructions: "fix the parser", Dir: work},
		runningNode("gpu-a", "org/model", nil),
		filepath.Join(work, "a.log"),
	)
	if err != nil {
		t.Fatalf("the launch should succeed: %v", err)
	}
	if err := child.Wait(); err != nil {
		t.Fatalf("the agent should end cleanly: %v", err)
	}

	// The kernel resolves the item's directory for the child, so the
	// reported working directory is the resolved one.
	canonical := work
	if resolved, err := filepath.EvalSymlinks(work); err == nil {
		canonical = resolved
	}
	rec := readRecord(t, record)
	for _, want := range []string{
		"cwd:" + canonical,
		"arg:run",
		"arg:-m",
		"arg:spinloop-orchestrator-gpu-a/org/model",
		"arg:fix the parser",
		"key:the-token",
	} {
		if !strings.Contains(rec, want) {
			t.Errorf("the agent should have been given %q, record:\n%s", want, rec)
		}
	}
	log, err := os.ReadFile(filepath.Join(work, "a.log"))
	if err != nil || !strings.Contains(string(log), "the agent's output") {
		t.Errorf("the agent's output should be kept per item beside the items file, got %q", log)
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.applied) != 1 {
		t.Fatalf("one apply per launch, got %d", len(h.applied))
	}
	ap := h.applied[0]
	if ap.sel.Provider != "spinloop-orchestrator-gpu-a" {
		t.Errorf("the provider block is keyed per node, got %q", ap.sel.Provider)
	}
	if ap.sel.Model != "org/model" || ap.sel.BaseURL != "http://gateway:4000" {
		t.Errorf("the selection names the node's model and the gateway, got %+v", ap.sel)
	}
	if ap.setDefault {
		t.Error("a dispatch does not make its model the harness's default")
	}
	if ap.keyEnv != "the-token" {
		t.Errorf("the config's key variable resolves to the gateway's token, got %q", ap.keyEnv)
	}
}

func TestDispatch_MissingDirectoryFailsNamingTheItem(t *testing.T) {
	h := &fakeHarness{name: "opencode", bin: stubAgent(t)}
	d := NewDispatcher(h, "http://gateway:4000", "the-token")
	_, err := d.Launch(
		Item{ID: "ghost", Instructions: "do", Dir: "/nonexistent/dir"},
		runningNode("n", "org/m", nil),
		filepath.Join(t.TempDir(), "ghost.log"),
	)
	if err == nil {
		t.Fatal("a missing directory should fail the launch")
	}
	if !strings.Contains(err.Error(), `item "ghost"`) || !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("the failure should name the item, got %v", err)
	}
	if h.applyCount() != 0 {
		t.Error("a failed launch applies nothing")
	}
}

func TestDispatch_AHarnessWithoutASingleTaskFormFailsNamingIt(t *testing.T) {
	h := &fakeHarness{name: "lucinate", bin: stubAgent(t)}
	d := NewDispatcher(h, "http://gateway:4000", "the-token")
	_, err := d.Launch(
		Item{ID: "a", Instructions: "do", Dir: t.TempDir()},
		runningNode("n", "org/m", nil),
		filepath.Join(t.TempDir(), "a.log"),
	)
	if err == nil {
		t.Fatal("a harness without a single-task form should fail the launch")
	}
	if !strings.Contains(err.Error(), `lucinate`) {
		t.Errorf("the failure should name the harness, got %v", err)
	}
}

func TestDispatch_ANodeWithNoModelFailsNamingIt(t *testing.T) {
	h := &fakeHarness{name: "opencode", bin: stubAgent(t)}
	d := NewDispatcher(h, "http://gateway:4000", "the-token")
	_, err := d.Launch(
		Item{ID: "a", Instructions: "do", Dir: t.TempDir()},
		stoppedNode("n", "", nil),
		filepath.Join(t.TempDir(), "a.log"),
	)
	if err == nil || !strings.Contains(err.Error(), "node n") {
		t.Errorf("a node with no model to run against should fail naming it, got %v", err)
	}
}

// --- 3.5: the loop ------------------------------------------------------------

// loopRun runs the loop in a goroutine and polls the state until cond holds
// or the deadline passes, then cancels and reports the loop's own error.
func loopRun(t *testing.T, cfg Config, cond func(stateFile) bool) error {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Run(ctx, cfg) }()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if cond(readState(t, cfg.ItemsPath)) {
			break
		}
		select {
		case err := <-done:
			t.Fatalf("the loop ended before the condition: %v", err)
		default:
		}
		time.Sleep(2 * time.Millisecond)
	}
	cancel()
	return <-done
}

// testConfig builds a loop config over the fixtures, with the items file at
// path. The returned config's ItemsPath is the file's path.
func testConfig(t *testing.T, topo *fakeTopo, h *fakeHarness, rec *launchRecorder, content string) (Config, string) {
	t.Helper()
	path := writeItems(t, content)
	d := NewDispatcher(h, "http://gateway:4000", "the-token")
	d.start = rec.start
	return Config{
		Gateway:    "http://gateway:4000",
		ItemsPath:  path,
		Topologist: topo,
		Dispatcher: d,
		Tick:       2 * time.Millisecond,
	}, path
}

func TestLoop_ItemsFlowBacklogRunningDone(t *testing.T) {
	topo := &fakeTopo{}
	topo.set(Topology{Wake: true, Nodes: []Node{runningNode("n", "org/m", nil)}})
	h := &fakeHarness{name: "opencode", bin: "unused"}
	rec := &launchRecorder{factory: func(bin string, args []string, dir, logPath string, env []string) (Child, error) {
		return newFakeChild(nil, nil), nil
	}}
	cfg, _ := testConfig(t, topo, h, rec, itemsFile(itemSpec{id: "a", dir: t.TempDir()}))

	err := loopRun(t, cfg, func(sf stateFile) bool {
		return sf.Items["a"].State == StateDone
	})
	if err != nil {
		t.Fatalf("the loop should end on the cancel, not an error: %v", err)
	}
	sf := readState(t, cfg.ItemsPath)
	if sf.Items["a"].State != StateDone || sf.Items["a"].Node != "n" {
		t.Errorf("the item should be recorded done on its node, got %+v", sf.Items["a"])
	}
	if rec.launchCount() != 1 {
		t.Errorf("one item, one launch, got %d", rec.launchCount())
	}
}

func TestLoop_TheTotalHoldsTheLine(t *testing.T) {
	topo := &fakeTopo{}
	topo.set(Topology{
		Wake:        true,
		Nodes:       []Node{runningNode("n", "org/m", nil)},
		Concurrency: &Concurrency{Total: total(2)},
	})
	h := &fakeHarness{name: "opencode", bin: "unused"}
	factory, end := holdingFactory()
	rec := &launchRecorder{factory: factory}
	var specs []itemSpec
	for i := 0; i < 5; i++ {
		specs = append(specs, itemSpec{id: fmt.Sprintf("a%d", i), dir: t.TempDir()})
	}
	cfg, _ := testConfig(t, topo, h, rec, itemsFile(specs...))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Run(ctx, cfg) }()

	// Wait until the total is in flight — and, within a few passes, no more.
	deadline := time.Now().Add(10 * time.Second)
	for rec.launchCount() < 2 && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond) // a few passes at the 2ms tick
	if got := rec.launchCount(); got != 2 {
		t.Fatalf("at the total of two, no further item is admitted, got %d launches", got)
	}
	if got := rec.maxInFlight(); got > 2 {
		t.Fatalf("in-flight never exceeds the total, got %d", got)
	}

	// End the first agent: exactly one more launch, and no further.
	end(0)
	deadline = time.Now().Add(10 * time.Second)
	for rec.launchCount() < 3 && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond)
	if got := rec.launchCount(); got != 3 {
		t.Fatalf("a finished item frees its slot, got %d launches", got)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("the loop should end on the cancel: %v", err)
	}
}

func TestLoop_ATagLimitHoldsItsOwnItems(t *testing.T) {
	topo := &fakeTopo{}
	topo.set(Topology{
		Wake:        true,
		Nodes:       []Node{runningNode("n", "org/m", map[string]string{"gpu": "a100"})},
		Concurrency: &Concurrency{Tags: map[string]int{"gpu=a100": 1}},
	})
	h := &fakeHarness{name: "opencode", bin: "unused"}
	factory, _ := holdingFactory()
	rec := &launchRecorder{factory: factory}
	items := itemsFile(
		itemSpec{id: "g1", dir: t.TempDir(), extra: []string{"  tags:", "    - gpu=a100"}},
		itemSpec{id: "g2", dir: t.TempDir(), extra: []string{"  tags:", "    - gpu=a100"}},
		itemSpec{id: "c1", dir: t.TempDir()},
	)
	cfg, _ := testConfig(t, topo, h, rec, items)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Run(ctx, cfg) }()

	// The tagged items hold their limit; the untagged one takes the rest.
	deadline := time.Now().Add(10 * time.Second)
	for rec.launchCount() < 2 && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond)
	if got := rec.launchCount(); got != 2 {
		t.Fatalf("the tag limit bounds its own items, the untagged is admitted, got %d launches", got)
	}
	sf := readState(t, cfg.ItemsPath)
	if sf.Items["g1"].State != StateRunning || sf.Items["c1"].State != StateRunning {
		t.Errorf("the first tagged and the untagged are in flight, got %+v", sf.Items)
	}
	if sf.Items["g2"].State != "" {
		t.Errorf("the second tagged item waits, got %+v", sf.Items["g2"])
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("the loop should end on the cancel: %v", err)
	}
}

func TestLoop_AdmitsByPriorityFirst(t *testing.T) {
	topo := &fakeTopo{}
	topo.set(Topology{
		Wake:        true,
		Nodes:       []Node{runningNode("n", "org/m", nil)},
		Concurrency: &Concurrency{Total: total(1)},
	})
	h := &fakeHarness{name: "opencode", bin: "unused"}
	factory, _ := holdingFactory()
	rec := &launchRecorder{factory: factory}
	items := itemsFile(
		itemSpec{id: "low", instr: "low work", dir: t.TempDir()},
		itemSpec{id: "high", instr: "high work", dir: t.TempDir(), extra: []string{"  priority: 10"}},
	)
	cfg, _ := testConfig(t, topo, h, rec, items)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Run(ctx, cfg) }()
	deadline := time.Now().Add(10 * time.Second)
	for rec.launchCount() < 1 && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	args := rec.launchArgs()
	if len(args) != 1 || len(args[0]) == 0 || args[0][len(args[0])-1] != "high work" {
		t.Fatalf("at a total of one, the higher priority goes first, got %v", args)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("the loop should end on the cancel: %v", err)
	}
}

func TestLoop_ANUnmatchedItemWaitsUntilANodeAppears(t *testing.T) {
	topo := &fakeTopo{}
	topo.set(Topology{Wake: true, Nodes: []Node{
		runningNode("cpu", "org/m", map[string]string{"cpu": "big"}),
	}})
	h := &fakeHarness{name: "opencode", bin: "unused"}
	rec := &launchRecorder{factory: func(bin string, args []string, dir, logPath string, env []string) (Child, error) {
		return newFakeChild(nil, nil), nil
	}}
	items := itemsFile(itemSpec{
		id: "a", dir: t.TempDir(), extra: []string{"  tags:", "    - gpu=a100"},
	})
	cfg, _ := testConfig(t, topo, h, rec, items)

	// No matching node: the item waits, not fails.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Run(ctx, cfg) }()
	time.Sleep(30 * time.Millisecond)
	if rec.launchCount() != 0 {
		t.Fatalf("an item nothing matches is not launched, got %d", rec.launchCount())
	}
	if st := readState(t, cfg.ItemsPath).Items["a"]; st.State == StateFailed {
		t.Fatalf("waiting is not failing, got %+v", st)
	}
	// A node carrying the tag appears: the item is admitted.
	topo.set(Topology{Wake: true, Nodes: []Node{
		runningNode("cpu", "org/m", map[string]string{"cpu": "big"}),
		runningNode("gpu", "org/g", map[string]string{"gpu": "a100"}),
	}})
	deadline := time.Now().Add(10 * time.Second)
	for rec.launchCount() < 1 && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	if rec.launchCount() != 1 {
		t.Fatalf("a node appearing in the topology admits the waiting item, got %d launches", rec.launchCount())
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("the loop should end on the cancel: %v", err)
	}
}

func TestLoop_AGatewayThatStopsAnsweringEndsTheRunNamingIt(t *testing.T) {
	topo := &fakeTopo{}
	topo.set(Topology{Wake: true, Nodes: []Node{runningNode("n", "org/m", nil)}})
	h := &fakeHarness{name: "opencode", bin: "unused"}
	rec := &launchRecorder{factory: func(bin string, args []string, dir, logPath string, env []string) (Child, error) {
		return newFakeChild(nil, nil), nil
	}}
	cfg, _ := testConfig(t, topo, h, rec, itemsFile(itemSpec{id: "a", dir: t.TempDir()}))

	topo.failWith(errors.New("connection refused"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := Run(ctx, cfg)
	if err == nil {
		t.Fatal("a gateway that does not answer should end the run")
	}
	if !strings.Contains(err.Error(), "http://gateway:4000") {
		t.Errorf("the failure should name the gateway, got %v", err)
	}
}

func TestLoop_AMidRunGatewayFailureEndsTheRunNamingIt(t *testing.T) {
	topo := &fakeTopo{}
	topo.set(Topology{Wake: true, Nodes: []Node{runningNode("n", "org/m", nil)}})
	h := &fakeHarness{name: "opencode", bin: "unused"}
	rec := &launchRecorder{factory: func(bin string, args []string, dir, logPath string, env []string) (Child, error) {
		return newFakeChild(nil, make(chan struct{})), nil
	}}
	cfg, _ := testConfig(t, topo, h, rec, itemsFile(itemSpec{id: "a", dir: t.TempDir()}))

	// Let the first pass work, then the gateway stops answering.
	go func() {
		time.Sleep(10 * time.Millisecond)
		topo.failWith(errors.New("connection refused"))
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := Run(ctx, cfg)
	if err == nil || !strings.Contains(err.Error(), "http://gateway:4000") {
		t.Errorf("a gateway that stops answering ends the run, naming it, got %v", err)
	}
}

func TestLoop_AMissingDirectoryFailsOneItemAlone(t *testing.T) {
	topo := &fakeTopo{}
	topo.set(Topology{Wake: true, Nodes: []Node{runningNode("n", "org/m", nil)}})
	h := &fakeHarness{name: "opencode", bin: "unused"}
	rec := &launchRecorder{factory: func(bin string, args []string, dir, logPath string, env []string) (Child, error) {
		return newFakeChild(nil, nil), nil
	}}
	items := itemsFile(
		itemSpec{id: "ghost", dir: "/nonexistent/dir"},
		itemSpec{id: "live", dir: t.TempDir()},
	)
	cfg, _ := testConfig(t, topo, h, rec, items)

	err := loopRun(t, cfg, func(sf stateFile) bool {
		return sf.Items["live"].State == StateDone && sf.Items["ghost"].State == StateFailed
	})
	if err != nil {
		t.Fatalf("the loop should end on the cancel: %v", err)
	}
	sf := readState(t, cfg.ItemsPath)
	if sf.Items["ghost"].State != StateFailed || !strings.Contains(sf.Items["ghost"].Why, `item "ghost"`) {
		t.Errorf("the missing directory fails the item, naming it, got %+v", sf.Items["ghost"])
	}
	if sf.Items["live"].State != StateDone {
		t.Errorf("the rest of the backlog goes on, got %+v", sf.Items["live"])
	}
}

func TestLoop_ANAgentThatEndsInErrorIsRecordedFailed(t *testing.T) {
	topo := &fakeTopo{}
	topo.set(Topology{Wake: true, Nodes: []Node{runningNode("n", "org/m", nil)}})
	h := &fakeHarness{name: "opencode", bin: "unused"}
	rec := &launchRecorder{factory: func(bin string, args []string, dir, logPath string, env []string) (Child, error) {
		return newFakeChild(errors.New("exit status 1"), nil), nil
	}}
	cfg, _ := testConfig(t, topo, h, rec, itemsFile(itemSpec{id: "a", dir: t.TempDir()}))

	err := loopRun(t, cfg, func(sf stateFile) bool {
		return sf.Items["a"].State == StateFailed
	})
	if err != nil {
		t.Fatalf("the loop should end on the cancel: %v", err)
	}
	if st := readState(t, cfg.ItemsPath).Items["a"]; st.State != StateFailed || st.Why == "" {
		t.Errorf("an agent that ends in error is recorded failed, naming why, got %+v", st)
	}
}

func TestLoop_ACleanInterruptRequeuesItsItems(t *testing.T) {
	topo := &fakeTopo{}
	topo.set(Topology{Wake: true, Nodes: []Node{runningNode("n", "org/m", nil)}})
	h := &fakeHarness{name: "opencode", bin: "unused"}
	var launched *fakeChild
	rec := &launchRecorder{factory: func(bin string, args []string, dir, logPath string, env []string) (Child, error) {
		c := newFakeChild(nil, make(chan struct{}))
		launched = c
		return c, nil
	}}
	items := itemsFile(
		itemSpec{id: "a", dir: t.TempDir()},
		itemSpec{id: "b", dir: t.TempDir()},
	)
	cfg, path := testConfig(t, topo, h, rec, items)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, cfg) }()
	// Both items in flight, then the interrupt.
	deadline := time.Now().Add(10 * time.Second)
	for rec.launchCount() < 2 && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("a clean interrupt ends the run without an error, got %v", err)
	}
	if !launched.wasStopped() {
		t.Error("the interrupt stops the agents it launched")
	}
	if sf := readState(t, path); len(sf.Items) != 0 {
		t.Errorf("re-queued items have no record: back in the backlog, got %+v", sf.Items)
	}

	// The next start picks the items up again.
	rec2 := &launchRecorder{factory: func(bin string, args []string, dir, logPath string, env []string) (Child, error) {
		return newFakeChild(nil, nil), nil
	}}
	d2 := NewDispatcher(h, "http://gateway:4000", "the-token")
	d2.start = rec2.start
	cfg2 := Config{
		Gateway:    cfg.Gateway,
		ItemsPath:  path,
		Topologist: topo,
		Dispatcher: d2,
		Tick:       2 * time.Millisecond,
	}
	err := loopRun(t, cfg2, func(sf stateFile) bool {
		return sf.Items["a"].State == StateDone && sf.Items["b"].State == StateDone
	})
	if err != nil {
		t.Fatalf("the next start should work the re-queued items: %v", err)
	}
	if rec2.launchCount() != 2 {
		t.Errorf("the next start picks both items up, got %d launches", rec2.launchCount())
	}
}

func TestLoop_ARestartDoesNotRerunACrashedItem(t *testing.T) {
	topo := &fakeTopo{}
	topo.set(Topology{Wake: true, Nodes: []Node{runningNode("n", "org/m", nil)}})
	items := itemsFile(itemSpec{id: "a", dir: t.TempDir()})
	path := writeItems(t, items)
	// The state a crash left: the item running when the process died.
	writeState(t, path, stateFile{Items: map[string]ItemState{
		"a": {State: StateRunning, Node: "n"},
	}})

	h := &fakeHarness{name: "opencode", bin: "unused"}
	rec := &launchRecorder{factory: func(bin string, args []string, dir, logPath string, env []string) (Child, error) {
		return newFakeChild(nil, nil), nil
	}}
	d := NewDispatcher(h, "http://gateway:4000", "the-token")
	d.start = rec.start
	cfg := Config{
		Gateway:    "http://gateway:4000",
		ItemsPath:  path,
		Topologist: topo,
		Dispatcher: d,
		Tick:       2 * time.Millisecond,
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, cfg) }()
	time.Sleep(30 * time.Millisecond) // a few passes
	if rec.launchCount() != 0 {
		t.Fatalf("a crashed item is not re-run, got %d launches", rec.launchCount())
	}
	sf := readState(t, path)
	if sf.Items["a"].State != StateFailed || !strings.Contains(sf.Items["a"].Why, "stopped while it was running") {
		t.Errorf("the crashed item is recorded failed, naming the interruption, got %+v", sf.Items["a"])
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("the loop should end on the cancel: %v", err)
	}
}

func TestLoop_ANewItemInTheFileEntersTheBacklog(t *testing.T) {
	topo := &fakeTopo{}
	topo.set(Topology{Wake: true, Nodes: []Node{runningNode("n", "org/m", nil)}})
	h := &fakeHarness{name: "opencode", bin: "unused"}
	rec := &launchRecorder{factory: func(bin string, args []string, dir, logPath string, env []string) (Child, error) {
		return newFakeChild(nil, nil), nil
	}}
	dir := t.TempDir()
	items := itemsFile(itemSpec{id: "a", dir: dir})
	cfg, path := testConfig(t, topo, h, rec, items)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Run(ctx, cfg) }()
	// Let the first item finish, then the file gains an item.
	deadline := time.Now().Add(10 * time.Second)
	for readState(t, path).Items["a"].State != StateDone && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	// A write that advances the mtime.
	time.Sleep(20 * time.Millisecond)
	os.WriteFile(path, []byte(items+itemsFile(itemSpec{id: "b", dir: dir})), 0o600)
	deadline = time.Now().Add(10 * time.Second)
	for readState(t, path).Items["b"].State != StateDone && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	if st := readState(t, path).Items["b"]; st.State != StateDone {
		t.Errorf("a new item in the file enters the backlog and runs, got %+v", st)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("the loop should end on the cancel: %v", err)
	}
}

func TestLoop_ANUnparseableFileStopsTheRunNamingTheFile(t *testing.T) {
	topo := &fakeTopo{}
	topo.set(Topology{Wake: true, Nodes: []Node{runningNode("n", "org/m", nil)}})
	h := &fakeHarness{name: "opencode", bin: "unused"}
	rec := &launchRecorder{factory: func(bin string, args []string, dir, logPath string, env []string) (Child, error) {
		return newFakeChild(nil, make(chan struct{})), nil
	}}
	dir := t.TempDir()
	items := itemsFile(itemSpec{id: "a", dir: dir})
	cfg, path := testConfig(t, topo, h, rec, items)

	go func() {
		time.Sleep(10 * time.Millisecond)
		os.WriteFile(path, []byte("not: [a list\n"), 0o600)
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := Run(ctx, cfg)
	if err == nil || !strings.Contains(err.Error(), path) {
		t.Errorf("a file the orchestrator cannot parse stops the run, naming the file, got %v", err)
	}
}

// the process-group plumbing a clean interrupt relies on
func TestStopGroup_ReachesTheProcessGroup(t *testing.T) {
	// A sleep in its own process group: the polite signal ends it.
	cmd := exec.Command("sleep", "30")
	setProcAttr(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	child := &procChild{cmd: cmd}
	child.Stop()
	err := cmd.Wait()
	if err == nil {
		t.Fatal("the sleep should have ended")
	}
	ws, ok := cmd.ProcessState.Sys().(syscall.WaitStatus)
	if !ok || ws.Signal() != syscall.SIGTERM {
		t.Errorf("the group's lead process should end on the signal, got %v", err)
	}
}
