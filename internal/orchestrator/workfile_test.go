package orchestrator

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRequestAbort_AMarkerStandsBesideTheFile(t *testing.T) {
	path := writeItems(t, itemsFile(itemSpec{id: "a", dir: t.TempDir()}))

	if ids, err := AbortsPending(path); err != nil || len(ids) != 0 {
		t.Fatalf("no markers before the ask, got %v (%v)", ids, err)
	}
	if err := RequestAbort(path, "a"); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(abortsDirFor(path), "a")
	if fi, err := os.Stat(marker); err != nil || fi.IsDir() {
		t.Fatalf("the marker %s should stand as a file: %v %v", marker, fi, err)
	}
	// A marker already standing is no fault: the ask is in, once.
	if err := RequestAbort(path, "a"); err != nil {
		t.Fatalf("a second ask for the same id is no fault: %v", err)
	}
	ids, err := AbortsPending(path)
	if err != nil || len(ids) != 1 || ids[0] != "a" {
		t.Fatalf("one marker for a, got %v (%v)", ids, err)
	}
	os.Remove(marker)
	if ids, err := AbortsPending(path); err != nil || len(ids) != 0 {
		t.Fatalf("the marker taken up, none left: %v (%v)", ids, err)
	}
}

// deadPID is a pid no process carries: a process started and waited on.
func deadPID(t *testing.T) int {
	t.Helper()
	cmd := exec.Command("true")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	return pid
}

func TestLockStatus_TheLockBesideTheFile(t *testing.T) {
	path := writeItems(t, itemsFile(itemSpec{id: "a", dir: t.TempDir()}))

	pid, live, err := LockStatus(path)
	if err != nil || pid != 0 || live {
		t.Fatalf("no lock, no holder: %d %v (%v)", pid, live, err)
	}

	lock := lockFileFor(path)
	if err := os.WriteFile(lock, []byte(fmt.Sprintf("%d", os.Getpid())), 0o600); err != nil {
		t.Fatal(err)
	}
	pid, live, err = LockStatus(path)
	if err != nil || pid != os.Getpid() || !live {
		t.Fatalf("a live holder: %d %v (%v)", pid, live, err)
	}

	dead := deadPID(t)
	if err := os.WriteFile(lock, []byte(fmt.Sprintf("%d", dead)), 0o600); err != nil {
		t.Fatal(err)
	}
	pid, live, err = LockStatus(path)
	if err != nil || pid != dead || live {
		t.Fatalf("a holder that is gone: %d %v (%v)", pid, live, err)
	}

	if err := os.WriteFile(lock, []byte("not a pid"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LockStatus(path); err == nil {
		t.Fatal("a lock that is not a pid is a fault the status names")
	}
}

func TestLoadStateFile_And_SaveStateFile(t *testing.T) {
	path := writeItems(t, itemsFile(itemSpec{id: "a", dir: t.TempDir()}))

	records, err := LoadStateFile(path)
	if err != nil || len(records) != 0 {
		t.Fatalf("no state file, an empty record: %v (%v)", records, err)
	}

	err = SaveStateFile(path, map[string]ItemState{
		"a": {State: StateDone, Node: "n", EndedAt: "2026-09-14T00:00:00Z"},
		"b": {State: StateFailed, Why: "it failed", EndedAt: "2026-09-14T00:00:00Z"},
	})
	if err != nil {
		t.Fatal(err)
	}
	records, err = LoadStateFile(path)
	if err != nil || len(records) != 2 {
		t.Fatalf("the record round-trips: %v (%v)", records, err)
	}
	if st := records["a"]; st.State != StateDone || st.Node != "n" || st.EndedAt != "2026-09-14T00:00:00Z" {
		t.Errorf("the done record stands: %+v", st)
	}
	if st := records["b"]; st.State != StateFailed || st.Why != "it failed" {
		t.Errorf("the failed record stands: %+v", st)
	}

	// A nil record is the empty one, not a fault.
	if err := SaveStateFile(path, nil); err != nil {
		t.Fatal(err)
	}
	if records, err = LoadStateFile(path); err != nil || len(records) != 0 {
		t.Fatalf("the nil record is the empty one: %v (%v)", records, err)
	}
}

func TestLogPathFor_TheLogsBesideTheFile(t *testing.T) {
	path := writeItems(t, itemsFile(itemSpec{id: "a", dir: t.TempDir()}))
	if got, want := LogPathFor(path, "a"), filepath.Join(path+".logs", "a.log"); got != want {
		t.Errorf("the kept output's path: %s (want %s)", got, want)
	}
}

func TestValidateItem_TheFields(t *testing.T) {
	if err := ValidateItem(Item{ID: "a", Instructions: "", Dir: t.TempDir()}); err == nil {
		t.Error("an item with no instructions is a fault")
	}
	if err := ValidateItem(Item{ID: "a", Instructions: "do"}); err == nil {
		t.Error("an item with no working directory is a fault")
	}
	if err := ValidateItem(Item{ID: "a", Instructions: "do", Dir: t.TempDir(), Tags: []string{"plain"}}); err == nil {
		t.Error("a tag that is not key=value is a fault")
	}
	if err := ValidateItem(Item{ID: "a", Instructions: "do", Dir: t.TempDir(), Tags: []string{"k=v", "k=2"}}); err == nil {
		t.Error("a tag key named twice is a fault")
	}
	if err := ValidateItem(Item{ID: "a", Instructions: "do", Dir: t.TempDir(), Tags: []string{"k=v", "n=2"}}); err != nil {
		t.Errorf("a well-formed item passes: %v", err)
	}
}

func TestSaveItems_AValidItemsFileThroughTheWrite(t *testing.T) {
	path := writeItems(t, itemsFile(itemSpec{id: "a", dir: t.TempDir()}))

	items, err := LoadItems(path)
	if err != nil {
		t.Fatal(err)
	}
	items = append(items, Item{ID: "b", Instructions: "do b", Dir: t.TempDir()})
	if err := SaveItems(path, items); err != nil {
		t.Fatal(err)
	}
	items, err = LoadItems(path)
	if err != nil {
		t.Fatalf("the file is a valid items file after the write: %v", err)
	}
	if len(items) != 2 || items[0].ID != "a" || items[1].ID != "b" {
		t.Errorf("both items stand, in the file's order: %+v", items)
	}

	dup := append(append([]Item{}, items...), Item{ID: "a", Instructions: "again", Dir: t.TempDir()})
	if err := SaveItems(path, dup); err == nil {
		t.Error("an id named twice is a fault the save refuses")
	}
	if items, err = LoadItems(path); err != nil || len(items) != 2 {
		t.Fatalf("the refused save leaves the file as it stood: %+v (%v)", items, err)
	}

	if err := SaveItems(path, []Item{{ID: "c", Dir: t.TempDir()}}); err == nil {
		t.Error("an item that fails the file's validation is refused")
	}
}

func TestJoin_TheFileJoinedWithTheRecord(t *testing.T) {
	items := []Item{
		{ID: "a", Instructions: "do a", Dir: "da"},
		{ID: "b", Instructions: "do b", Dir: "db", Tags: []string{"k=v"}, Priority: 3},
		{ID: "c", Instructions: "do c", Dir: "dc"},
	}
	records := map[string]ItemState{
		"b": {State: StateRunning, Node: "n", StartedAt: "2026-09-14T00:00:00Z"},
		"c": {State: StateFailed, Why: "it failed", EndedAt: "2026-09-14T00:00:01Z"},
	}

	views := Join(items, records)
	if len(views) != 3 {
		t.Fatalf("one view per item, in the file's order: %+v", views)
	}
	a, b, c := views[0], views[1], views[2]
	if a.State != StateBacklog || a.Node != "" || a.StartedAt != "" || a.EndedAt != "" {
		t.Errorf("no record is backlog, nothing else: %+v", a)
	}
	if b.State != StateRunning || b.Node != "n" || b.StartedAt != "2026-09-14T00:00:00Z" ||
		b.Tags == nil || b.Priority != 3 {
		t.Errorf("the running record joins its item: %+v", b)
	}
	if c.State != StateFailed || c.Why != "it failed" || c.EndedAt != "2026-09-14T00:00:01Z" {
		t.Errorf("the failed record joins its item: %+v", c)
	}
}
