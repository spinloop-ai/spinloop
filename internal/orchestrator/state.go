package orchestrator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// The item states an item's record takes. Backlog has no record: an item
// with no entry is waiting.
const (
	StateBacklog = "backlog"
	StateRunning = "running"
	StateDone    = "done"
	StateFailed  = "failed"
)

// ItemState is one item's record: where it is, and why it is there.
type ItemState struct {
	// State is one of running, done, failed — backlog has no record.
	State string `json:"state"`
	// Why is the record's reason: the failure's text for a failed item,
	// the node's name for a running one.
	Why  string `json:"why,omitempty"`
	Node string `json:"node,omitempty"`
	// StartedAt and EndedAt are RFC 3339, set as the item moves.
	StartedAt string `json:"startedAt,omitempty"`
	EndedAt   string `json:"endedAt,omitempty"`
}

// stateFile is the state as written beside the items file.
type stateFile struct {
	Items map[string]ItemState `json:"items"`
}

// Store is the state an orchestrator works from beside its items file: the
// record of what happened to each item, the per-item logs, and the lock that
// keeps a second orchestrator off the same file. The file-backed store
// beside the items file is the default implementation; the run and the work
// list API both work through this seam, and a test needs no state on disk.
type Store interface {
	// Load reads the record: every item's ItemState, keyed by the item's id.
	Load() (map[string]ItemState, error)
	// Save writes the record, the way a torn write must never leave a
	// restart unsure which items were running.
	Save(items map[string]ItemState) error
	// LogPath is where one item's agent output is kept.
	LogPath(id string) string
	// Close releases the store's hold on the items file.
	Close()
}

// The files the state keeps beside one items file, named from the items
// path alone, so the run and a command beside the file reach the same ones.

func stateFileFor(itemsPath string) string { return itemsPath + ".state.json" }

func lockFileFor(itemsPath string) string { return itemsPath + ".lock" }

func logDirFor(itemsPath string) string { return itemsPath + ".logs" }

func abortsDirFor(itemsPath string) string { return itemsPath + ".aborts" }

// ItemWorkspaceDir is the harness's own working directory for an item: a
// `workspace` subdirectory of the item's own directory (the `dir` the items
// file names), bind-mounted into the container under the docker backend,
// and the bare backend's own process working directory.
func ItemWorkspaceDir(itemDir string) string {
	return filepath.Join(itemDir, "workspace")
}

// ItemConfigDir is where the docker backend renders one item's scoped,
// per-launch harness config: a `config` subdirectory of the item's own
// directory, alongside `workspace` — the item's own directory is the
// storage location for everything the orchestrator keeps about that one
// item's launch.
func ItemConfigDir(itemDir string) string {
	return filepath.Join(itemDir, "config")
}

// LogPathFor is where one item's kept output stands beside the items file.
func LogPathFor(itemsPath, id string) string {
	return filepath.Join(logDirFor(itemsPath), id+".log")
}

// fileStore is the state beside one items file: the state file, the log
// directory, and the lock.
type fileStore struct {
	statePath string
	logsDir   string
	lockPath  string
	pid       int
}

// OpenStore opens the state beside the items file: it takes the file's
// lock, refusing a second orchestrator for the same file, and performs the
// recovery a restart owes — an item the state left running is recorded
// failed, naming the interruption, and is not re-run.
func OpenStore(itemsPath string) (Store, error) {
	statePath := stateFileFor(itemsPath)
	lockPath := lockFileFor(itemsPath)

	pid, err := takeLock(lockPath)
	if err != nil {
		return nil, err
	}
	s := &fileStore{
		statePath: statePath,
		logsDir:   logDirFor(itemsPath),
		lockPath:  lockPath,
		pid:       pid,
	}
	if err := os.MkdirAll(s.logsDir, 0o700); err != nil {
		s.dropLock()
		return nil, fmt.Errorf("opening the log directory beside %s: %v", itemsPath, err)
	}
	items, err := s.Load()
	if err != nil {
		s.dropLock()
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	changed := false
	for id, st := range items {
		if st.State != StateRunning {
			continue
		}
		st.State = StateFailed
		st.Why = "the orchestrator stopped while it was running"
		st.EndedAt = now
		items[id] = st
		changed = true
	}
	if changed {
		if err := s.Save(items); err != nil {
			s.dropLock()
			return nil, err
		}
	}
	return s, nil
}

// Close releases the file's lock.
func (s *fileStore) Close() { s.dropLock() }

// takeLock claims the lock file for this process. A lock held by a live
// process is refused, naming the holder; a lock whose process is gone is
// taken over.
func takeLock(path string) (int, error) {
	pid := os.Getpid()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err == nil {
		f.WriteString(fmt.Sprintf("%d", pid))
		f.Close()
		return pid, nil
	}
	if !os.IsExist(err) {
		return 0, fmt.Errorf("taking the lock beside the work items file: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("reading the lock held beside the work items file: %v", err)
	}
	var holder int
	if _, err := fmt.Sscanf(string(data), "%d", &holder); err == nil && alive(holder) {
		return 0, fmt.Errorf("another orchestrator (pid %d) is working this items file", holder)
	}
	if err := os.Remove(path); err != nil {
		return 0, fmt.Errorf("taking over a stale lock beside the work items file: %v", err)
	}
	f, err = os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, fmt.Errorf("taking over the lock beside the work items file: %v", err)
	}
	f.WriteString(fmt.Sprintf("%d", pid))
	f.Close()
	return pid, nil
}

// dropLock releases the lock file, the way a clean exit leaves it: gone.
func (s *fileStore) dropLock() {
	os.Remove(s.lockPath)
}

// LoadStateFile reads the record beside the items file without taking the
// lock and without the recovery a start performs: a command beside the file
// reads what is there, and a record left running stands until a start makes
// it failed. An absent file is an empty record.
func LoadStateFile(itemsPath string) (map[string]ItemState, error) {
	return readStateFile(stateFileFor(itemsPath))
}

// SaveStateFile writes the record beside the items file, atomically, without
// taking the lock: the command's write, the way the run's save is one — a
// torn write must never leave a restart unsure which items were running.
func SaveStateFile(itemsPath string, items map[string]ItemState) error {
	if items == nil {
		items = map[string]ItemState{}
	}
	return writeStateFile(stateFileFor(itemsPath), stateFile{Items: items})
}

// LockStatus reports the lock's holder and whether the holder is alive, an
// absent lock being no holder: the read-only check a command beside the file
// makes before it waits on a run, the liveness the run's takeover judges by.
func LockStatus(itemsPath string) (pid int, live bool, err error) {
	data, err := os.ReadFile(lockFileFor(itemsPath))
	if err != nil {
		if os.IsNotExist(err) {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("reading the lock beside the work items file: %v", err)
	}
	if _, err := fmt.Sscanf(string(data), "%d", &pid); err != nil {
		return 0, false, fmt.Errorf("the lock beside the work items file is not a pid: %v", err)
	}
	return pid, alive(pid), nil
}

// Load reads the record, an absent file being an empty one.
func (s *fileStore) Load() (map[string]ItemState, error) {
	return readStateFile(s.statePath)
}

// save writes the record as the file's state, atomically.
func (s *fileStore) save(sf stateFile) error {
	return writeStateFile(s.statePath, sf)
}

// readStateFile reads the record at the state's path, an absent file being
// an empty one.
func readStateFile(path string) (map[string]ItemState, error) {
	var sf stateFile
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]ItemState{}, nil
		}
		return nil, fmt.Errorf("reading the state beside the work items file: %v", err)
	}
	if err := json.Unmarshal(data, &sf); err != nil {
		return nil, fmt.Errorf("the state beside the work items file is not a record: %v", err)
	}
	if sf.Items == nil {
		sf.Items = map[string]ItemState{}
	}
	return sf.Items, nil
}

// writeStateFile writes the record to the state's path, atomically: a torn
// write must never leave a restart unsure which items were running.
func writeStateFile(path string, sf stateFile) error {
	data, err := json.MarshalIndent(sf, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// Save writes the record, atomically.
func (s *fileStore) Save(items map[string]ItemState) error {
	if items == nil {
		items = map[string]ItemState{}
	}
	return s.save(stateFile{Items: items})
}

// LogPath is where one item's agent output is kept, beside the items file.
func (s *fileStore) LogPath(id string) string {
	return filepath.Join(s.logsDir, id+".log")
}
