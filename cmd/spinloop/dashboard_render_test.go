package main

import (
	"strings"
	"testing"
)

// A wrapped line is the terminal's own break: hard at the column, wide
// clusters whole on the next row, and a line the pane can hold arriving as
// the single row it always was.
func TestDashWrapLines(t *testing.T) {
	cases := []struct {
		name  string
		line  string
		width int
		want  []string
	}{
		{"fits as one row", "short", 10, []string{"short"}},
		{"exact multiple", "abcdefgh", 4, []string{"abcd", "efgh"}},
		{"remainder row", "abcdefghij", 4, []string{"abcd", "efgh", "ij"}},
		{"wide cluster moves whole", "a漢b", 2, []string{"a", "漢", "b"}},
		{"empty line stays a row", "", 10, []string{""}},
		{"zero width draws the line whole", "漢漢", 0, []string{"漢漢"}},
		{"pane narrower than one wide glyph", "漢漢", 1, []string{"漢漢"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := dashWrapLines(tc.line, tc.width)
			if len(got) != len(tc.want) {
				t.Fatalf("rows = %q, want %q", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("rows = %q, want %q", got, tc.want)
				}
			}
		})
	}
}

// Nothing is lost or duplicated across the break: the rows reassemble the
// line exactly.
func TestDashWrapLinesLosesNothing(t *testing.T) {
	line := strings.Repeat("漢x", 60)
	rows := dashWrapLines(line, 7)
	if strings.Join(rows, "") != line {
		t.Errorf("rejoining the rows did not recover the line:\n%q", rows)
	}
}

// Off is today's clip: one row per line, cut at the pane's width.
func TestDashLogRowsClippedVersusWrapped(t *testing.T) {
	lines := []string{strings.Repeat("x", 10), "y"}

	wrapped := dashLogRows(lines, 4, true)
	if len(wrapped) != 4 || wrapped[0] != "xxxx" || wrapped[3] != "y" {
		t.Errorf("wrapped rows = %q", wrapped)
	}

	clipped := dashLogRows(lines, 4, false)
	if len(clipped) != 2 || clipped[0] != "xxxx" || clipped[1] != "y" {
		t.Errorf("clipped rows = %q", clipped)
	}
}
