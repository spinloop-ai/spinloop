// Environment references in a Spinloop's values ($FOO, ${FOO}, ${FOO:-bar},
// $$ — see internal/envref) resolve when a command consumes them, never when
// the file is parsed, shown or exported. PARAM values have their own choke
// point (selectionParams, named by key on failure); ENV values resolve where
// they are exported (applySpinloopEnv). This file holds the choke point for
// everything else: the scalar instruction values.

package main

import (
	"fmt"

	"github.com/spinloop-ai/spinloop/internal/envref"
	"github.com/spinloop-ai/spinloop/internal/spinloop"
)

// scalarKeywords pairs each scalar Selection field with the keyword that
// names it in an error, in the canonical form `export` renders.
var scalarKeywords = []struct {
	keyword string
	field   func(*spinloop.Selection) *string
}{
	{"PROVIDER", func(s *spinloop.Selection) *string { return &s.Provider }},
	{"MODEL", func(s *spinloop.Selection) *string { return &s.Model }},
	{"ALIAS", func(s *spinloop.Selection) *string { return &s.Alias }},
	{"CONTEXT", func(s *spinloop.Selection) *string { return &s.Context }},
	{"OUTPUT", func(s *spinloop.Selection) *string { return &s.Output }},
	{"PARALLEL", func(s *spinloop.Selection) *string { return &s.Parallel }},
	{"BASEURL", func(s *spinloop.Selection) *string { return &s.BaseURL }},
	{"PRESET", func(s *spinloop.Selection) *string { return &s.Preset }},
}

// resolveSelection returns a copy of sel with every environment reference in
// its scalar instruction values — PROVIDER, MODEL, ALIAS, CONTEXT, OUTPUT,
// PARALLEL, BASEURL and PRESET — resolved against the process environment as
// the caller holds it. Every consuming command calls it once, right after
// reading the Spinloop (and after applySpinloopEnv on the paths that layer
// the file's own environment first), so the resolved text is what every
// later interpretation sees: the catalogue lookup of a PROVIDER, the size
// parse of a CONTEXT, the path resolution of a PRESET, the repo:quant split
// of a MODEL. A malformed reference fails the command naming its keyword,
// rather than passing raw text onward.
//
// Params and Env are deliberately untouched: selectionParams owns PARAM
// resolution (so a caller that derives from PARAMs alone — or a test that
// calls it directly — still resolves through the one choke point), and
// applySpinloopEnv owns ENV values at the moment they are exported. The
// families are disjoint, so no value is ever expanded twice.
func resolveSelection(sel spinloop.Selection) (spinloop.Selection, error) {
	out := sel
	for _, kw := range scalarKeywords {
		value, err := envref.Expand(*kw.field(&out))
		if err != nil {
			return spinloop.Selection{}, fmt.Errorf("%s: %w", kw.keyword, err)
		}
		*kw.field(&out) = value
	}
	return out, nil
}
