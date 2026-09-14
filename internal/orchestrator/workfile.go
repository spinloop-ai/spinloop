// The file-side surface the work commands work through: the items file a
// command reads and rewrites, the abort marker a command leaves for the run
// beside the file, and the view the file's items joined with the run's
// record. The run owns the lock and the commands never take it; the run's
// pass re-reads the file and makes the state agree with it.
package orchestrator

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ValidateItem is the item's fields through the file's validation: the
// present instructions and working directory, and the tags that are
// well-formed key=value pairs, no key named twice.
func ValidateItem(item Item) error {
	return validateItemFields(item.ID, item.Instructions, item.Dir, item.Tags)
}

// SaveItems writes the items to the items file, the way the run writes it:
// atomically, and the result validated the way the run re-reads it, so the
// file is a valid items file through the write.
func SaveItems(path string, items []Item) error {
	files := make([]fileItem, 0, len(items))
	for _, it := range items {
		files = append(files, fileItemFromItem(it))
	}
	data, err := yaml.Marshal(files)
	if err != nil {
		return fmt.Errorf("writing the work items file %s: %v", path, err)
	}
	if _, err := ParseItems(data); err != nil {
		return fmt.Errorf("the work items file %s: %v", path, err)
	}
	return writeItemsFileData(path, data)
}

// RequestAbort asks the run working the file to abort the item: the marker
// beside the file the run's pass takes up. A marker already standing is no
// fault: the ask is in, and the run takes it up once.
func RequestAbort(itemsPath, id string) error {
	dir := abortsDirFor(itemsPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("writing the abort marker beside the work items file: %v", err)
	}
	f, err := os.OpenFile(filepath.Join(dir, id), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err == nil {
		f.Close()
		return nil
	}
	if os.IsExist(err) {
		return nil
	}
	return fmt.Errorf("writing the abort marker beside the work items file: %v", err)
}

// AbortsPending is the ids with a marker standing beside the file, an absent
// directory being none.
func AbortsPending(itemsPath string) ([]string, error) {
	entries, err := os.ReadDir(abortsDirFor(itemsPath))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading the abort markers beside the work items file: %v", err)
	}
	ids := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			ids = append(ids, e.Name())
		}
	}
	return ids, nil
}

// Join is the file's items, in file order, each with its record — backlog
// where there is none: the view the run's list and the work list command
// render, one fact worded from one place.
func Join(items []Item, records map[string]ItemState) []ItemView {
	out := make([]ItemView, 0, len(items))
	for _, it := range items {
		v := ItemView{
			ID: it.ID, Instructions: it.Instructions, Dir: it.Dir,
			Tags: it.Tags, Priority: it.Priority, State: StateBacklog,
		}
		if st, ok := records[it.ID]; ok {
			v.State = st.State
			v.Node = st.Node
			v.Why = st.Why
			v.StartedAt = st.StartedAt
			v.EndedAt = st.EndedAt
		}
		out = append(out, v)
	}
	return out
}
