package cloud

import (
	"strings"
	"testing"
)

// The CLI's name grammar is the plane's: anything the control plane's
// Lambdas would refuse must be refused here, early, naming the shape — not
// arrive at the plane as an anonymous HTTP 400.
func TestIsNodeNameMatchesThePlaneGrammar(t *testing.T) {
	good := []string{
		"a", "0", "9", "Dev", "dev", "qwen3.6-27b", "lab_01", "my-node",
		"dev.example.internal", strings.Repeat("a", 64),
	}
	for _, name := range good {
		if !IsNodeName(name) {
			t.Errorf("%q should be a node name", name)
		}
	}
	bad := []string{
		"", " ", "my node", "my\tnode", ".hidden", "-lead", "_lead",
		"/lead", "trail/", "back\\slash", "uniço", "a b", "a\nb",
		"prod.json", strings.Repeat("a", 65),
	}
	for _, name := range bad {
		if IsNodeName(name) {
			t.Errorf("%q should not be a node name", name)
		}
	}
}

func TestNotNodeName_NamesTheShape(t *testing.T) {
	err := NotNodeName("my node")
	if err == nil {
		t.Fatal("want an error")
	}
	for _, want := range []string{"my node", "starts with a letter or digit", "64"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

// A name refused at the CLI must say what a name looks like, not just that
// this one is not one — the error is the documentation of the grammar.
func TestNotNodeName_IsTheSingleWording(t *testing.T) {
	// ResolveNode is one of the refusing sites; its error must be NotNodeName's,
	// so the instruction never differs by which command caught the mistake.
	_, err := ResolveNode(nil, "my node", PlaneRequest{})
	if err == nil || err.Error() != NotNodeName("my node").Error() {
		t.Errorf("ResolveNode error = %v, want the shared wording", err)
	}
}
