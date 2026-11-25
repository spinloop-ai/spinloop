package cloud

import (
	"fmt"
	"regexp"
	"strings"
)

// nodeNamePattern is the plane's own name grammar: a plain identifier of up
// to 64 characters, starting with a letter or digit and continuing with
// letters, digits, dots, underscores or dashes. It is deliberately the same
// grammar the control plane's Lambdas enforce, so nothing a plane would
// refuse ever reaches it — a name that cannot be a node name is refused by
// the CLI naming the shape, not by an anonymous HTTP 400 from the plane.
var nodeNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// IsNodeName reports whether a value is a plain node name in the grammar the
// control plane accepts, plus the one rule the CLI keeps for itself: a name
// may not end in `.json`.
//
// A cloud node is named, never addressed: the name is the key the control
// plane stores its deploy config, key secret and tagged resources under, and
// the key a fleet file's `kind: cloud` node is keyed by. So a `--node` value,
// or a fleet node name, that is not a plain identifier is refused here rather
// than reaching a plane or a path. The `.json` suffix is the plane's to
// accept and the CLI's to refuse: a value ending that way is a file reference
// pasted where a name was asked for, and answering it as a name would only
// send the caller hunting for a node nobody saves as a filename. Names are
// plane-global — the same name on two planes is two nodes, and on one plane
// it is one node however many people use it.
func IsNodeName(value string) bool {
	if strings.HasSuffix(value, ".json") {
		return false
	}
	return nodeNamePattern.MatchString(value)
}

// NotNodeName words the refusal of a value that is not a node name, naming
// the shape it would have to have. Every command refusing a name uses this,
// so the instruction never differs by which command caught the mistake.
func NotNodeName(value string) error {
	return fmt.Errorf(
		"%q is not a node name: a node name is a plain identifier — it starts with a letter or digit, "+
			"continues with letters, digits, dots, underscores or dashes, and is at most 64 characters", value)
}
