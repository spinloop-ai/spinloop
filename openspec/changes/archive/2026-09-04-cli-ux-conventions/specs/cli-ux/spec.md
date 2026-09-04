## Purpose

How the command-line tool and its full-screen views look, what they write to
which stream, how they word what they say, and what they must keep saying while
they work. It is what makes a command added later recognisable as part of the
same tool without its author having to read the others first.

It is the counterpart to the web repo's `design-language` spec, not a copy of
it. A terminal has affordances a page does not — a stdout another program may
be parsing, an output stream that may not be a terminal at all, lines that
scroll away rather than staying to be re-read, a screen redrawn in place — and
has none of what that spec governs: typefaces, breakpoints, layout gaps, vector
icons.

## ADDED Requirements

### Requirement: One accent colour, and it never reports a state

The tool SHALL use a single brand accent — the mint of the spinloop logo,
`#1DE2AD`, the value the site's accent token carries — defined once and derived
from that definition everywhere it is used. It SHALL be used only where nothing
about an engine, a node or an operation is being reported: the product's name,
the mark on the thing the operator has selected, and no more.

Colours that report a state — the green, amber and red of a resource bar or a
health mark — SHALL NOT be used for the tool's own chrome, and the accent SHALL
NOT be used for a state. The two are read differently: a state colour answers
"how is this going", the accent answers "which tool is this", and a surface
that uses one for the other makes both unreadable.

The tool SHALL be legible on a light terminal as well as a dark one. Text that
carries no meaning of its own SHALL be left in the terminal's own foreground
colour rather than set to a near-white or near-black of the tool's choosing.

#### Scenario: The accent marks a selection, not a state

- **WHEN** a full-screen view marks which item the operator has selected
- **THEN** it uses the accent, and no colour that reports what that item is
  doing

#### Scenario: A state keeps the terminal's own state colours

- **WHEN** a resource bar, a health mark or an outcome is drawn
- **THEN** its colour is the green, amber or red that reports what it is
  reporting, and never the brand accent

#### Scenario: Changing the accent re-tints the tool

- **WHEN** the accent's definition is changed
- **THEN** every accented surface changes with it, because no surface carries
  its own copy of the value

### Requirement: A stdout a program consumes carries nothing else

A command whose stdout carries a machine-readable result — the exports an
`eval` consumes, a `--format=json` document — SHALL write that result to stdout
and everything else to stderr: progress, explanation and warnings. A command
whose whole output is a report for a person MAY write that report to stdout,
which is what stdout is for, and SHALL keep prompts and warnings off it.

A command whose stdout is being consumed SHALL NOT be made to say less: what it
reports on stderr does not change with where its stdout goes.

#### Scenario: A pipeline gets only what it can parse

- **WHEN** a command that prints both progress and a machine-readable result is
  piped into another program
- **THEN** only the result reaches that program, and the progress is still
  shown to the person who ran it

#### Scenario: A prompt does not land in a capture

- **WHEN** a command asks the operator a question
- **THEN** the question is written to stderr, so a captured stdout holds the
  command's output and not its conversation

#### Scenario: A shell-completion request stays silent

- **WHEN** the shell asks the tool for completions
- **THEN** the tool writes completions and nothing else, whatever the state of
  its configuration

### Requirement: Decoration is drawn only where it can be seen

Colour, spinners, cursor movement and in-place redrawing SHALL be used only
when the stream being written to is a terminal. A run whose output is
redirected — to a file, to a log, to CI — SHALL get plain lines in the same
order, rather than a file full of escape codes.

This is what keeps a spinner out of a capture, whichever stream it is drawn
on: a redirected stream is not a terminal, so nothing is drawn on it.

A full-screen view SHALL refuse to start when it has no terminal to draw on,
and SHALL name the command that reports the same information into a pipe.

#### Scenario: A redirected run gets plain lines

- **WHEN** a command that draws a spinner has its output redirected to a file
- **THEN** the file holds the same information as plain lines, with no spinner
  and no escape codes

#### Scenario: A full-screen view refuses a pipe

- **WHEN** a full-screen view is invoked with its output piped
- **THEN** it fails before drawing anything, and its error names the command
  that carries the same information into a pipe

### Requirement: A long operation keeps saying what it is doing

An operation that can take longer than a few seconds SHALL report its current
situation, and SHALL replace that report outright at each transition rather
than adding to it: what is shown is what is happening now, never what was
happening before.

Everything such a report says about time SHALL be computed when it is drawn or
written, not when the situation arose — a wait counts down towards what it is
waiting for, and elapsed time counts up. A situation that holds unchanged for
minutes is the normal case, so a moving value is what distinguishes an
operation that is waiting from one that has stopped making progress. Where the
surface redraws in place, an operation in flight SHALL also carry a spinner.

The tool SHALL use one spinner, defined once, so every surface that shows work
in progress shows the same thing.

#### Scenario: A wait counts down as it is watched

- **WHEN** an operation is waiting for a retry and the operator watches without
  pressing anything
- **THEN** the time until that retry counts down, and the time the operation
  has been running counts up

#### Scenario: A superseded situation is not left on screen

- **WHEN** an operation moves on from one situation to another
- **THEN** what is shown is the new one, with no trace of the one it replaced

### Requirement: An error names what to do about it

An error SHALL be a lowercase phrase with no trailing full stop. It SHALL quote
the value the operator supplied, name the file, flag or environment variable
it concerns, and — where a command would fix it — give that command.

An error SHALL be reported without a usage dump: the failure is what the
operator needs to read, and burying it under the command's full help hides it.

#### Scenario: An unknown name says what the known ones are

- **WHEN** the operator names something that does not exist
- **THEN** the error quotes what they typed, says where it was looked for, and
  either lists what is there or names the command that would

#### Scenario: A broken reference names its repair

- **WHEN** a stored reference points at something that has gone
- **THEN** the error says what it points at and gives the command that would
  re-point or remove it

### Requirement: Help text is a lowercase imperative phrase

A command's one-line description SHALL be a lowercase phrase in the
imperative, without a trailing full stop, naming what the command does rather
than what it is. A flag's usage string SHALL follow the same form. A command's
longer description SHALL add what the short one could not carry, and SHALL NOT
repeat it.

#### Scenario: A new command reads like the others

- **WHEN** a command is added
- **THEN** its description is a lowercase imperative phrase with no trailing
  full stop, as its neighbours' are

### Requirement: Copy is plain, specific and British

Everything the tool writes — help, errors, progress, prompts and the text
inside a full-screen view — SHALL use ordinary English in British spelling. It
SHALL name the actual thing rather than describe it abstractly, and SHALL use
technical vocabulary only where it is more precise or more concise than plain
wording.

Where the same fact is reported by more than one surface, those surfaces SHALL
word it from one place rather than each writing their own version, so two
screens cannot describe one situation differently.

#### Scenario: British spelling throughout

- **WHEN** any text the tool prints is written
- **THEN** it uses British spellings

#### Scenario: Two surfaces cannot word one fact differently

- **WHEN** the same fact is shown both in a full-screen view and by a one-shot
  command
- **THEN** both draw their wording from the same place

### Requirement: A destructive action asks first, and can be told not to

An action that destroys or replaces something the operator cannot trivially
recreate SHALL ask for confirmation before it is sent, and SHALL offer a flag
that skips the question for an unattended run. The question SHALL be written to
stderr and SHALL default to not proceeding, so a bare newline or a closed input
declines.

A declined or abandoned confirmation SHALL send nothing and SHALL say that
nothing was done.

#### Scenario: A declined confirmation changes nothing

- **WHEN** the operator is asked to confirm a destructive action and declines
- **THEN** nothing is sent, and the tool says so

#### Scenario: An unattended run is not blocked by a prompt

- **WHEN** a destructive action is run with the flag that skips confirmation
- **THEN** it proceeds without asking

### Requirement: A full-screen view offers only keys that would do something

A full-screen view SHALL name its keys on screen, and SHALL name a key only
where pressing it would do something in the context that key help describes. A
key that would do nothing for what is currently selected SHALL NOT be
advertised there.

Such a view SHALL make plain how current what it is showing is: information
that has aged well past the rate it is refreshed at SHALL be shown with its age
rather than drawn identically to information just read, and SHALL NOT be
reported as the present state of anything.

#### Scenario: The key help drops a key that would do nothing

- **WHEN** the selected item has nothing for a given key to act on
- **THEN** the key help does not name that key
- **WHEN** the selection moves to an item that key does act on
- **THEN** the key help names it

#### Scenario: Aged information says how old it is

- **WHEN** what a panel shows has not been refreshed for well past its own
  refresh rate
- **THEN** the panel shows how old it is and stops presenting it as current
