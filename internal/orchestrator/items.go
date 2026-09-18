// The work items file: the backlog the orchestrator works. A file is a list
// of items in file order; each names the work its agent is given, its own
// directory (see Item.Dir), and — where the item is bound to a kind of
// node — the tags of the nodes it can run on.
package orchestrator

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/spinloop-ai/spinloop/internal/fleet"
)

// Item is one unit of work from the items file.
type Item struct {
	// ID is the item's name within the file: unique, and the key of its
	// record in the state beside the file.
	ID string
	// Instructions is what the item's agent is told to do.
	Instructions string
	// Dir is the item's own directory: the storage location for everything
	// the orchestrator keeps about that item's launch — the harness's own
	// working directory under a `workspace` subdirectory (ItemWorkspaceDir),
	// and, under the docker backend, its scoped config under a `config`
	// subdirectory (ItemConfigDir).
	Dir string
	// Tags are the nodes the item can run on, named the way node tags are
	// named — a key and a value joined by =. An item that names none
	// matches any node.
	Tags []string
	// Priority ranks the backlog: higher first. It defaults to the lowest,
	// and among equals file order wins.
	Priority int
}

// fileItem is one item as written in the file.
type fileItem struct {
	ID           string   `yaml:"id"`
	Instructions string   `yaml:"instructions"`
	Dir          string   `yaml:"dir"`
	Tags         []string `yaml:"tags"`
	// Priority is a pointer, so a stated zero — the lowest rank — is told
	// apart from one the file never stated.
	Priority *int `yaml:"priority"`
}

// LoadItems reads and validates the work items file at path.
func LoadItems(path string) ([]Item, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	items, err := ParseItems(data)
	if err != nil {
		return nil, fmt.Errorf("the work items file %s: %v", path, err)
	}
	return items, nil
}

// ParseItems parses and validates the file's contents: a list of items, each
// with a unique id, instructions, and a working directory. A file the
// orchestrator cannot parse is an error naming the fault, so the command
// stops before it works an item.
func ParseItems(data []byte) ([]Item, error) {
	var file []fileItem
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("not a list of items: %v", err)
	}
	items := make([]Item, 0, len(file))
	seen := map[string]int{}
	for i, f := range file {
		name := f.ID
		if name == "" {
			name = fmt.Sprintf("item %d", i+1)
		}
		if f.ID == "" {
			return nil, fmt.Errorf("%s has no id", name)
		}
		if at, dup := seen[f.ID]; dup {
			return nil, fmt.Errorf("duplicate id %q (items %d and %d)", f.ID, at+1, i+1)
		}
		seen[f.ID] = i
		if err := checkItem(f); err != nil {
			return nil, err
		}
		priority := 0
		if f.Priority != nil {
			priority = *f.Priority
		}
		items = append(items, Item{
			ID:           f.ID,
			Instructions: f.Instructions,
			Dir:          f.Dir,
			Tags:         f.Tags,
			Priority:     priority,
		})
	}
	return items, nil
}

// checkItem is the validation one item's fields pass: a present id,
// instructions, and working directory, and tags that are well-formed key
// =value pairs, no key named twice. The file's parse applies it, and the
// work list API's add and the work add command apply the same.
func checkItem(f fileItem) error {
	return validateItemFields(f.ID, f.Instructions, f.Dir, f.Tags)
}

// validateItemFields is the fields' validation, the item's form agnostic:
// the instructions and working directory present, and the tags that are
// well-formed key=value pairs, no key named twice.
func validateItemFields(id, instructions, dir string, tags []string) error {
	if instructions == "" {
		return fmt.Errorf("item %q has no instructions", id)
	}
	if dir == "" {
		return fmt.Errorf("item %q has no working directory", id)
	}
	for _, tag := range tags {
		if _, _, ok := fleet.SplitTag(tag); !ok {
			return fmt.Errorf("item %q's tag %q is not a key=value pair", id, tag)
		}
	}
	if keys := duplicateTagKeys(tags); keys != "" {
		return fmt.Errorf("item %q names the tag key%s more than once", id, keys)
	}
	return nil
}

// fileItemFromItem is the item's file form: the priority rendered where it
// is above the lowest rank, absent where it is the lowest.
func fileItemFromItem(it Item) fileItem {
	f := fileItem{ID: it.ID, Instructions: it.Instructions, Dir: it.Dir, Tags: it.Tags}
	if it.Priority > 0 {
		p := it.Priority
		f.Priority = &p
	}
	return f
}

// duplicateTagKeys reports the tag keys an item names more than once, as a
// comma-separated suffix for the error: a node cannot carry two values for
// one key, so such an item would match nothing and wait forever.
func duplicateTagKeys(tags []string) string {
	seen := map[string]bool{}
	var dups []string
	for _, tag := range tags {
		key, _, _ := fleet.SplitTag(tag)
		if seen[key] {
			dups = append(dups, key)
			continue
		}
		seen[key] = true
	}
	if len(dups) == 0 {
		return ""
	}
	list := ""
	for i, key := range dups {
		if i > 0 {
			list += ", "
		}
		list += fmt.Sprintf("%q", key)
	}
	return list
}

// sortedForAdmission orders the backlog the way the orchestrator works it:
// highest priority first, and among equals, file order.
func sortedForAdmission(items []Item) []Item {
	out := make([]Item, len(items))
	copy(out, items)
	// The file order is the position in items, so a stable sort on priority
	// alone keeps it among equals.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].Priority > out[j-1].Priority; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
