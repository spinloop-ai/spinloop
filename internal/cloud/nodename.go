package cloud

import (
	"strings"
)

// IsNodeName reports whether a value is a plain node name: non-empty,
// with no path separator and no .json suffix.
//
// A cloud node is named, never addressed: the name is the key the control
// plane stores its deploy config, key secret and tagged resources under, and
// the key a fleet file's `kind: cloud` node is keyed by. So a `--node` value,
// or a fleet node name, that is not a plain identifier is refused here rather
// than reaching a plane or a path. Names are plane-global — the same name on
// two planes is two nodes, and on one plane it is one node
// however many people use it.
func IsNodeName(value string) bool {
	if value == "" {
		return false
	}
	if strings.ContainsAny(value, `/\`) {
		return false
	}
	if strings.HasSuffix(value, ".json") {
		return false
	}
	return true
}
