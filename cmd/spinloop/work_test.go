package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spinloop-ai/spinloop/internal/orchestrator"
)

// runWork works the work command group the way the binary does, the output
// kept for the test to read: the stdout the command wrote, and its error.
func runWork(t *testing.T, args ...string) (string, error) {
	t.Helper()
	stdout := filepath.Join(t.TempDir(), "stdout")
	f, err := os.Create(stdout)
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = f
	defer func() {
		os.Stdout = old
		f.Close()
	}()
	err = cmdWork(args)
	data, _ := os.ReadFile(stdout)
	return string(data), err
}

// workFileIn writes a work items file and its state into a fresh directory.
func workFileIn(t *testing.T, items, state string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "work.yaml")
	mustWrite(t, path, items)
	if state != "" {
		mustWrite(t, path+".state.json", state)
	}
	return path
}

func TestWorkAdd_AppendsTheItem(t *testing.T) {
	path := workFileIn(t, "- id: a\n  instructions: do a\n  dir: .\n", "")
	out, err := runWork(t, "add", "--items", path, "--id", "b", "--instructions", "do b", "--dir", ".")
	if err != nil {
		t.Fatalf("the add: %v (out %s)", err, out)
	}
	if !strings.Contains(out, `item "b" added`) {
		t.Errorf("the add reports the item: %s", out)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "id: a") || !strings.Contains(string(data), "id: b") {
		t.Errorf("both items stand in the file:\n%s", data)
	}
}

func TestWorkAdd_CreatesAMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "work.yaml")
	if _, err := runWork(t, "add", "--items", path, "--id", "a", "--instructions", "do", "--dir", "."); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the missing file is created: %v", err)
	}
	if !strings.Contains(string(data), "id: a") {
		t.Errorf("the created file carries the item:\n%s", data)
	}
}

func TestWorkAdd_Refusals(t *testing.T) {
	_, err := runWork(t, "add", "--items", workFileIn(t, "", ""), "--instructions", "do", "--dir", ".")
	if err == nil || !strings.Contains(err.Error(), "--id") {
		t.Errorf("an add with no id is refused, naming the flag: %v", err)
	}

	path := workFileIn(t, "- id: a\n  instructions: do a\n  dir: .\n", "")
	_, err = runWork(t, "add", "--items", path, "--id", "a", "--instructions", "again", "--dir", ".")
	if err == nil || !strings.Contains(err.Error(), `id "a"`) {
		t.Errorf("an id the file carries is refused, naming it: %v", err)
	}

	// The id out of the file, its ended record in the state: the state's
	// refusal, not the file's.
	ended := workFileIn(t, "- id: b\n  instructions: do b\n  dir: .\n",
		`{"items":{"a":{"state":"done","node":"n","endedAt":"2026-09-14T00:00:00Z"}}}`)
	_, err = runWork(t, "add", "--items", ended, "--id", "a", "--instructions", "again", "--dir", ".")
	if err == nil || !strings.Contains(err.Error(), "done") {
		t.Errorf("an id the state records done is refused, naming the record: %v", err)
	}
}

func TestWorkList_PlainLinesInFileOrder(t *testing.T) {
	state := `{"items":{` +
		`"b":{"state":"running","node":"n","startedAt":"2026-09-14T00:00:00Z"},` +
		`"c":{"state":"done","node":"n","startedAt":"2026-09-14T00:00:00Z","endedAt":"2026-09-14T00:01:00Z"},` +
		`"d":{"state":"failed","why":"it failed","endedAt":"2026-09-14T00:02:00Z"}}}`
	path := workFileIn(t,
		"- id: a\n  instructions: do a\n  dir: .\n"+
			"- id: b\n  instructions: do b\n  dir: .\n"+
			"- id: c\n  instructions: do c\n  dir: .\n"+
			"- id: d\n  instructions: do d\n  dir: .\n",
		state)

	out, err := runWork(t, "list", "--items", path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("one line per item:\n%s", out)
	}
	if lines[0] != "a\tbacklog\t-\t-\t-" {
		t.Errorf("no record is backlog, dashes where the values are absent: %q", lines[0])
	}
	if lines[1] != "b\trunning\tn\t2026-09-14T00:00:00Z\t-" {
		t.Errorf("the running record joins its item: %q", lines[1])
	}
	if lines[2] != "c\tdone\tn\t2026-09-14T00:00:00Z\t2026-09-14T00:01:00Z" {
		t.Errorf("the done record joins its item: %q", lines[2])
	}
	if lines[3] != "d\tfailed\t-\t-\t2026-09-14T00:02:00Z" {
		t.Errorf("the failed record joins its item: %q", lines[3])
	}
	// No state colour off a terminal.
	if strings.Contains(out, "\033[") {
		t.Errorf("no colour where there is no terminal:\n%q", out)
	}

	// The alias works the same list.
	out, err = runWork(t, "ls", "--items", path)
	if err != nil || !strings.HasPrefix(out, "a\tbacklog") {
		t.Errorf("the ls alias is the list: %v\n%s", err, out)
	}
}

func TestWorkList_NoStateFileEverythingIsBacklog(t *testing.T) {
	path := workFileIn(t, "- id: a\n  instructions: do a\n  dir: .\n", "")
	out, err := runWork(t, "list", "--items", path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "a\tbacklog\t-\t-\t-\n") {
		t.Errorf("no state file, everything backlog:\n%s", out)
	}
}

func TestWorkList_MissingFileIsANamedError(t *testing.T) {
	_, err := runWork(t, "list", "--items", filepath.Join(t.TempDir(), "work.yaml"))
	if err == nil || !strings.Contains(err.Error(), "no work items file") {
		t.Errorf("a missing file is refused, naming it: %v", err)
	}
}

func TestWorkListLine_TheStateColourWhereThereIsATerminal(t *testing.T) {
	v := orchestratorItemView("a", "done", "", "", "")
	if line := workListLine(v, true); !strings.Contains(line, ansiGreen) || !strings.Contains(line, ansiReset) {
		t.Errorf("a terminal draws the done state in its colour: %q", line)
	}
	if line := workListLine(v, false); strings.Contains(line, "\033[") {
		t.Errorf("no terminal, no colour: %q", line)
	}
	v = orchestratorItemView("a", "failed", "", "", "")
	if line := workListLine(v, true); !strings.Contains(line, ansiRed) {
		t.Errorf("the failed state is its colour: %q", line)
	}
	v = orchestratorItemView("a", "running", "n", "2026-09-14T00:00:00Z", "")
	if line := workListLine(v, true); !strings.Contains(line, ansiYellow) {
		t.Errorf("the running state is its colour: %q", line)
	}
	v = orchestratorItemView("a", "backlog", "", "", "")
	if line := workListLine(v, true); strings.Contains(line, "\033[") {
		t.Errorf("the backlog state takes no colour: %q", line)
	}
}

func TestWorkAbort_Refusals(t *testing.T) {
	missing := workFileIn(t, "- id: a\n  instructions: do a\n  dir: .\n", "")
	_, err := runWork(t, "abort", "b", "--items", missing)
	if err == nil || !strings.Contains(err.Error(), `no item with id "b"`) {
		t.Errorf("an id the file does not carry is refused, naming it: %v", err)
	}

	backlog := workFileIn(t, "- id: a\n  instructions: do a\n  dir: .\n", "")
	_, err = runWork(t, "abort", "a", "--items", backlog)
	if err == nil || !strings.Contains(err.Error(), "it is backlog") {
		t.Errorf("a backlog item is refused, naming its state: %v", err)
	}

	done := workFileIn(t, "- id: a\n  instructions: do a\n  dir: .\n",
		`{"items":{"a":{"state":"done","node":"n","endedAt":"2026-09-14T00:00:00Z"}}}`)
	_, err = runWork(t, "abort", "a", "--items", done)
	if err == nil || !strings.Contains(err.Error(), "it is done") {
		t.Errorf("a done item is refused, naming its state: %v", err)
	}

	// A running record with no orchestrator holding the lock: nothing is
	// running, and the command says so — the record stands for the next start.
	running := workFileIn(t, "- id: a\n  instructions: do a\n  dir: .\n",
		`{"items":{"a":{"state":"running","node":"n","startedAt":"2026-09-14T00:00:00Z"}}}`)
	_, err = runWork(t, "abort", "a", "--items", running)
	if err == nil || !strings.Contains(err.Error(), "no orchestrator is running") {
		t.Errorf("no live lock, nothing is running, and the command says so: %v", err)
	}
}

func TestWorkAbort_TheMarkerGoesInAndTheWaitComesBack(t *testing.T) {
	running := workFileIn(t, "- id: a\n  instructions: do a\n  dir: .\n",
		`{"items":{"a":{"state":"running","node":"n","startedAt":"2026-09-14T00:00:00Z"}}}`)
	// The orchestrator's lock, held by this process: the wait has a run to
	// wait on.
	if err := os.WriteFile(running+".lock", []byte(itoa(os.Getpid())), 0o600); err != nil {
		t.Fatal(err)
	}
	// The run's take-up, a moment out: the record goes, then the marker.
	marker := filepath.Join(running+".aborts", "a")
	go func() {
		time.Sleep(300 * time.Millisecond)
		os.WriteFile(running+".state.json", []byte(`{"items":{}}`), 0o600)
		os.Remove(marker)
	}()

	tick := workWaitTick
	workWaitTick = 5 * time.Millisecond
	defer func() { workWaitTick = tick }()

	out, err := runWork(t, "abort", "a", "--items", running)
	if err != nil {
		t.Fatalf("the abort: %v (out %s)", err, out)
	}
	if !strings.Contains(out, "back in the backlog") {
		t.Errorf("the wait comes back with the item's state: %s", out)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Errorf("the marker is taken up, none standing: %v", err)
	}
}

func TestWorkRemove_TakesTheItemOut(t *testing.T) {
	log := workFileIn(t,
		"- id: a\n  instructions: do a\n  dir: .\n"+
			"- id: b\n  instructions: do b\n  dir: .\n",
		`{"items":{"a":{"state":"done","node":"n","endedAt":"2026-09-14T00:00:00Z"}}}`)
	logDir := log + ".logs"
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(logDir, "a.log"), []byte("the output"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := runWork(t, "remove", "a", "--items", log)
	if err != nil {
		t.Fatal(err)
	}
	items, err := orchestratorLoadItems(log)
	if err != nil {
		t.Fatalf("the file is a valid items file after the removal: %v", err)
	}
	if len(items) != 1 || items[0].ID != "b" {
		t.Errorf("the item is out of the file, the rest stands: %+v", items)
	}
	if _, err := os.Stat(filepath.Join(logDir, "a.log")); !os.IsNotExist(err) {
		t.Errorf("the kept output goes with the item: %v", err)
	}
	state, _ := os.ReadFile(log + ".state.json")
	if strings.Contains(string(state), `"a"`) {
		t.Errorf("the record is out of the state:\n%s", state)
	}
}

func TestWorkRemove_Refusals(t *testing.T) {
	missing := workFileIn(t, "- id: a\n  instructions: do a\n  dir: .\n", "")
	_, err := runWork(t, "remove", "b", "--items", missing)
	if err == nil || !strings.Contains(err.Error(), `no item with id "b"`) {
		t.Errorf("an id the file does not carry is refused, naming it: %v", err)
	}

	running := workFileIn(t, "- id: a\n  instructions: do a\n  dir: .\n",
		`{"items":{"a":{"state":"running","node":"n","startedAt":"2026-09-14T00:00:00Z"}}}`)
	_, err = runWork(t, "remove", "a", "--items", running)
	if err == nil || !strings.Contains(err.Error(), "abort it first") {
		t.Errorf("a running item is refused, naming the abort that goes first: %v", err)
	}
}

func TestWorkRemove_TheWaitComesBackWhereTheRunIsLive(t *testing.T) {
	path := workFileIn(t,
		"- id: a\n  instructions: do a\n  dir: .\n"+
			"- id: b\n  instructions: do b\n  dir: .\n",
		`{"items":{"a":{"state":"done","node":"n","endedAt":"2026-09-14T00:00:00Z"}}}`)
	if err := os.WriteFile(path+".lock", []byte(itoa(os.Getpid())), 0o600); err != nil {
		t.Fatal(err)
	}
	// The run's pass drops the record a moment out: the wait has a run to
	// wait on, and it comes back when the state agrees.
	go func() {
		time.Sleep(300 * time.Millisecond)
		os.WriteFile(path+".state.json", []byte(`{"items":{}}`), 0o600)
	}()

	tick := workWaitTick
	workWaitTick = 5 * time.Millisecond
	defer func() { workWaitTick = tick }()

	out, err := runWork(t, "remove", "a", "--items", path)
	if err != nil {
		t.Fatalf("the remove: %v (out %s)", err, out)
	}
	if !strings.Contains(out, `item "a" removed`) {
		t.Errorf("the remove reports the item: %s", out)
	}
}

// orchestratorItemView builds a work list view for the line's tests.
func orchestratorItemView(id, state, node, started, ended string) orchestrator.ItemView {
	return orchestrator.ItemView{ID: id, State: state, Node: node, StartedAt: started, EndedAt: ended}
}

// orchestratorLoadItems reads an items file the way the commands do.
func orchestratorLoadItems(path string) ([]orchestrator.Item, error) {
	return orchestrator.LoadItems(path)
}

// itoa renders a pid the way the lock file carries it.
func itoa(n int) string { return fmt.Sprintf("%d", n) }
