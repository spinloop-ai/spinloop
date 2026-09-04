## Context

The conventions this records are already in the code. A survey of
`cmd/spinloop` before writing the spec found them held consistently, with two
duplications and one slip:

| convention | where it is already held |
| --- | --- |
| machine-readable output on stdout, everything else on stderr | `remote start`'s exports against its progress; `fleet status`/`metrics` writing their tables and JSON to stdout |
| decoration only on a terminal | `fleet deploy`'s spinner and `fleet dashboard`'s refusal both check `term.IsTerminal` |
| errors lowercase, quoting the value, naming the command that fixes it | `main.go`'s alias and provider errors; `fleet.go`'s unknown-node error |
| help as a lowercase imperative phrase | every `Short:` in the command tree |
| confirmation with a `--yes` escape | `remote bootstrap` |
| one accent, used only for chrome | the dashboard's title bar and selected border |
| a long operation reporting a situation that moves | `remote start`'s phases, and the dashboard tile rendering the same ones |

The duplications: the ten braille spinner frames are written out in
`fleet.go` and again in `dashboard_model.go`, in the same package. The slip:
one `Long:` description in `fleet.go` says "behavior".

So the work is mostly writing the spec. The code changes are the two things
writing it down makes obviously wrong.

## Findings from the survey

Reviewing the drafted spec against `cmd/spinloop`, command by command:

- **`fleet deploy` draws its spinner on stdout, not stderr.** The first draft
  of the stdout requirement listed spinners among the things that go to stderr,
  which would have made this a violation. It is not one: the spinner is drawn
  only when stdout is a terminal, so a redirected or piped run never sees it,
  and `fleet deploy` has no machine-readable stdout for it to corrupt. The
  requirement was reworded to bite where it matters — a stdout carrying a
  result a program consumes — and the rule about redrawing was left with the
  requirement that already owns it. This is a correction to the spec, not a
  finding against the code.
- **One American spelling.** `fleet deploy`'s long description says "behavior".
  It is the only one in the package, and it is fixed here rather than recorded,
  so the spec is not landed already violated.
- **Two definitions of the spinner.** `deploySpinnerFrames` in `fleet.go` and
  `dashSpinnerFrames` in `dashboard_model.go` hold the same ten braille frames.
  Consolidated by this change.
- **Everything else holds.** Every `Short:` in the command tree is a lowercase
  imperative phrase with no trailing full stop, and so is every flag usage
  string. Errors are lowercase, quote the operator's value with `%q`, and name
  the command that fixes them. `remote start` keeps its exports on stdout and
  its progress on stderr. `remote bootstrap` confirms before it creates
  anything and takes `--yes`. The dashboard's accent is used only for its title
  bar and its selection.

## Goals / Non-Goals

**Goals:**

- A spec a reviewer can point at, and an author of a new command can read
  instead of guessing from a neighbour.
- Record the brand accent's value in this repo, so it need not be looked up in
  the web repo.
- Remove the duplicate spinner definition, and put the palette where a second
  command would look for it.

**Non-Goals:**

- Changing any wording the CLI currently prints, beyond the one American
  spelling. Where a command and the spec disagree, that is a finding for
  someone to raise later.
- Restating what `remote-metrics-bar-format` and `fleet-client` already
  specify. Those keep the resource bars' colour thresholds and the dashboard's
  own behaviour; `cli-ux` states the general rules they are instances of.
- A shared internal package for terminal rendering. Everything here lives in
  `package main` under `cmd/spinloop`, which is one package already.

## Decisions

### Decision 1: one capability, not one per surface

`cli-ux` covers the whole tool — one-shot commands and the full-screen view
alike — because the conventions are what the two have in common. Splitting it
per surface would put "errors are lowercase" in one spec and the same sentence
in another, which is the situation this change exists to end.

The full-screen affordances that have no one-shot equivalent (key help,
staleness) are a requirement within it rather than a separate capability, since
the rule is one sentence each and both are about the same tool.

**Alternative rejected:** folding these into `fleet-client`. That spec is about
one command's behaviour; a rule for every command does not belong under it, and
a second command wanting the same rule would have to reference a spec it has
nothing else to do with.

### Decision 2: the spec names the accent's value, not its source

`#1DE2AD` is written into the spec, with a note that it is the value the site's
accent token carries. The alternative — "the accent is whatever the web repo's
token says" — would make this repo's spec unreadable without the other one to
hand, and would not be checkable here.

The two are kept in step by being the same short hex value in two places, both
naming the logo they were sampled from. That is a copy, and copies drift; the
cost of the alternative is worse.

### Decision 3: the accent is defined by what it may not do

The requirement is not "use mint for highlights" but "use it only where nothing
about a node is being reported, and never use a state colour for chrome". The
prohibition is the useful half: the failure it prevents is a board where the
selected panel and a panel wanting attention are the same colour, which is what
the selection border was before this.

### Decision 4: one spinner, in the file the palette moves to

The frames and the brand colours are the two things every surface that draws
anything needs, so both go in one small file, and the two current definitions
of the frames become one. Nothing else moves.

### Decision 5: British spelling is a requirement, and the one slip is fixed

The rule is stated, and the single `Long:` description that says "behavior" is
corrected in the same change, so the spec is not written already violated. That
is the only wording this change touches.

## Risks / Trade-offs

- **A spec recording current practice can be read as blessing it.** Some of
  what is written down is simply what was done first. The requirements are
  worded as rules rather than as descriptions, so a later change that wants to
  do differently has to argue with the rule rather than quietly diverge.
- **The accent's value is duplicated across two repos.** Accepted, with both
  copies naming the logo as their source. A shared token file across two repos
  with different build systems would cost more than the drift it prevents.
- **The spec is broad and the tests for it are indirect.** Most of these
  requirements are already pinned by existing tests of the commands they apply
  to; a few (help wording, British spelling) are conventions a reviewer checks.
  Adding a linter for those is possible and is not in this change.
