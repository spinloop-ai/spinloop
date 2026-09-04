## Why

The CLI's conventions exist, are followed fairly consistently, and are written
down nowhere. What goes on stdout and what goes on stderr, how an error is
worded, when a spinner is drawn, which colour means what — each was decided
once and has since been copied from whatever nearby code the next command was
modelled on. That works while there is nearby code to copy; it fails the first
time someone adds a surface with no neighbour, and it gives a reviewer nothing
to point at.

The web repo has a `design-language` spec for the site. This repo has no
counterpart, and cannot simply adopt that one: a CLI has affordances a web page
does not — a stdout that may be piped into another program, a terminal that may
not be a terminal, output that scrolls away rather than being re-read, a full
screen redrawn in place — and lacks most of what that spec governs (typefaces,
breakpoints, layout gaps, vector icons).

The dashboard work that precedes this made the gap concrete: the brand accent
had to be looked up in another repo, and the same spinner is now defined twice
in one package because there was nothing saying there should be one.

## What Changes

- **A `cli-ux` capability records the conventions.** One accent colour and the
  rule that it is only ever used where nothing about a node is being reported;
  the split between what a program may parse and what a person reads; when
  colour, spinners and in-place redraws are drawn at all; how an error is
  worded; British spelling; what a long operation must keep saying; what a
  destructive action must ask; and the affordances that belong to a full-screen
  view.
- **The brand palette is stated in this repo.** The accent is the mint of the
  spinloop logo, the same value the site's `--accent` token carries. The spec
  records the value and, more importantly, records that the terminal's own
  green, amber and red are not part of it — they report an engine's state and
  are specified by `remote-metrics-bar-format`.
- **The spinner is defined once.** `deploySpinnerFrames` in `fleet.go` and
  `dashSpinnerFrames` in `dashboard_model.go` are the same ten braille frames
  written out twice, in one package. One definition replaces both.
- **The brand colours move to their own file.** They are currently in
  `dashboard_render.go`, which is where they were first needed rather than
  where a second command would look for them.

No command's behaviour changes. The spec records what the CLI already does, and
the two code changes are consolidations that follow from writing it down.

## Capabilities

### New Capabilities

- `cli-ux`: how the CLI and its full-screen views look, what they write where,
  how they word what they say, and what they must keep saying while they work.

### Modified Capabilities

(None. `remote-metrics-bar-format` keeps the resource bars' colour thresholds
and `fleet-client` keeps the dashboard's own behaviour; `cli-ux` states the
general rules those two are instances of, and does not restate either.)

## Impact

- Spec: a new `openspec/specs/cli-ux/spec.md`.
- Code: `cmd/spinloop/fleet.go` and `cmd/spinloop/dashboard_model.go` share one
  spinner definition; the brand colours move out of `dashboard_render.go` into
  a file of their own. No behaviour changes, and the existing tests pin that.
- Docs: `AGENTS.md`'s spec pointers, and a note in `docs/internals.md` on where
  the palette lives.
- Not in scope: changing any wording the CLI currently prints. Where the spec
  and a command disagree, that is a finding to raise, not something this change
  fixes.
