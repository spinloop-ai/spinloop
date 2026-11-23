package envref

import (
	"strings"
	"testing"
)

func TestExpand_Forms(t *testing.T) {
	t.Setenv("FOO", "bar")
	t.Setenv("EMPTY", "")
	t.Setenv("A_B2", "x")

	cases := []struct {
		name, in, want string
	}{
		{"no dollar untouched", "unsloth/Qwen3.8:Q4_K_M", "unsloth/Qwen3.8:Q4_K_M"},
		{"empty", "", ""},
		{"bare name", "$FOO/x", "bar/x"},
		{"braced", "${FOO}/x", "bar/x"},
		{"bare name at end", "repo:$FOO", "repo:bar"},
		{"digits after name stop the name", "${FOO}1", "bar1"},
		{"underscore start", "$_UNSET_SEES_EMPTY", ""},
		{"mixed", "a$FOO${FOO}b", "abarbarb"},
		// Unset or empty variables.
		{"unset bare", "$NOPE!", "!"},
		{"unset braced", "x${NOPE}y", "xy"},
		{"set but empty", "$EMPTY", ""},
		// Defaults: used when unset or empty, unused when set non-empty.
		{"default stands when unset", "${NOPE:-fallback}", "fallback"},
		{"default stands when empty", "${EMPTY:-fallback}", "fallback"},
		{"default ignored when set", "${FOO:-fallback}", "bar"},
		{"empty default valid", "${NOPE:-}", ""},
		{"default may hold punctuation", "${NOPE:-a:-b/c}", "a:-b/c"},
		// Escapes and stray dollars.
		{"escaped dollar", "$$5", "$5"},
		{"escaped before a name", "$$FOO", "$FOO"},
		{"escaped before a brace", "$${FOO}", "${FOO}"},
		{"double escape twice", "$$$$FOO", "$$FOO"},
		{"stray dollar of non-name", "100%$?", "100%$?"},
		{"dollar at end", "cost$", "cost$"},
		{"dollar before digit", "$5", "$5"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Expand(tc.in)
			if err != nil {
				t.Fatalf("Expand(%q) error: %v", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("Expand(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestExpand_Errors(t *testing.T) {
	t.Setenv("FOO", "bar")
	cases := []struct{ name, in, wantSub string }{
		{"unterminated", "org/model:${HF_TAG", "unterminated"},
		{"unterminated later in the value", "a ${A b", "unterminated"},
		{"empty name", "${}", "invalid"},
		{"name starts with a digit", "${1FOO}", "invalid"},
		{"name holds a space", "${A B}", "invalid"},
		{"default with a bad name", "${1FOO:-x}", "invalid"},
		{"plus form is not defined", "${FOO:+x}", "unsupported"},
		{"length form is not defined", "${#FOO}", "invalid"},
		{"bare colon form", "${FOO:1}", "unsupported"},
		{"colon at end with no dash", "${FOO:}", "unsupported"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Expand(tc.in)
			if err == nil {
				t.Fatalf("Expand(%q) should fail", tc.in)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("error %q should contain %q", err, tc.wantSub)
			}
			// cli-ux: lowercase phrase, names the fix.
			if !strings.Contains(err.Error(), "$$") {
				t.Errorf("error %q should point at the $$ escape", err)
			}
		})
	}
}
