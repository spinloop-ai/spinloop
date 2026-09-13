# Delta: alias-registry

## MODIFIED Requirements

### Requirement: Alias resolution

Wherever a Spinloop path is accepted, an argument SHALL be looked up in the
registry only when it is name-shaped — a path-shaped or URL-shaped argument
never causes a registry read at all, so commands keep working when spinloop's
own config is absent or unreadable. A path on disk SHALL beat a registered
name of the same spelling, and the shadowing SHALL be reported, not silent. A
registered name whose target is a local file that no longer exists SHALL fail
with instructions to re-point or drop the alias; a registered name whose
target is a URL SHALL NOT be probed for liveness during resolution — a
network failure surfaces normally, at the point the target is actually
fetched. When an alias decides the path, the command SHALL say so.

That report SHALL go to stderr. It is prose about how the command was resolved
rather than the command's result, and the same resolution serves
`spinloop remote env`, whose stdout is meant to be evaluated by a shell.

#### Scenario: Alias used from anywhere

- **WHEN** the user runs `spinloop harness apply qwen3.6-27b` in an unrelated directory
- **THEN** the registered Spinloop is applied and the output names the alias and
  the resolved path

#### Scenario: The alias note stays out of stdout

- **WHEN** an alias resolves the Spinloop for a command whose stdout is consumed
  by a shell, such as `spinloop remote env`
- **THEN** the note naming the alias is written to stderr and stdout carries
  only the command's own output

#### Scenario: Path beats alias

- **WHEN** an argument names both a file on disk and a registered alias
- **THEN** the file wins and a note reports that the path was used

#### Scenario: Dangling alias

- **WHEN** a registered name points at a local file that has been deleted
- **THEN** the command fails suggesting `spinloop alias -n <name> <path>` or
  `spinloop unalias <name>`

#### Scenario: A URL alias is not probed before use

- **WHEN** a registered name points at a URL and the user runs
  `spinloop harness apply <name>`
- **THEN** resolution proceeds without a preliminary network check; the
  Spinloop is fetched directly, and a failure there (unreachable host, non-2xx
  status) is reported as an ordinary fetch error

### Requirement: Naming an alias in the environment

The `SPINLOOP_ALIAS` environment variable SHALL name a registered alias, and that
alias SHALL be used by any command that takes a Spinloop path but was given
none. It SHALL rank below an explicit argument and above the `./Spinloop`
default, so a command that names a path or an alias is unaffected.

The value SHALL be treated as a registry name only, never as a path: it SHALL
be looked up in the registry directly, and a file of the same name in the
working directory SHALL NOT shadow it. This is the opposite of the rule for an
argument, and deliberate — an argument is usually a path, whereas the variable
can only have been set to name an alias.

An empty or unset `SPINLOOP_ALIAS` SHALL have no effect. A value that is not
name-shaped, is not registered, or points at a file that no longer exists SHALL
fail naming `SPINLOOP_ALIAS` as the source, so the variable is never mistaken for
a missing file in the current directory.

When `SPINLOOP_ALIAS` decides the Spinloop, the command SHALL say so on stderr,
naming the variable, the alias and the resolved path.

A command that consults a Spinloop only when there is one to consult — the
`remote` subcommands, which otherwise act on the `default` environment, and
`daemon`, which otherwise starts idle — SHALL count `SPINLOOP_ALIAS` as naming
one. A set variable SHALL NOT be passed over in favour of that fallback.

#### Scenario: The variable counts as having a Spinloop

- **WHEN** `SPINLOOP_ALIAS` names a Spinloop whose `ENV` instructions set
  `AWS_PROFILE` and the user runs `spinloop remote status` in a directory with
  no `Spinloop`
- **THEN** that Spinloop's `ENV` instructions are applied to the process
  environment before the control call, rather than being skipped because no
  `Spinloop` sits in the working directory

#### Scenario: The variable supplies the Spinloop

- **WHEN** `SPINLOOP_ALIAS=qwen3.6-27b` is set and the user runs `spinloop harness apply`
  in a directory with no `Spinloop`
- **THEN** the registered Spinloop is applied and a note on stderr names the
  variable, the alias and the resolved path

#### Scenario: An argument wins

- **WHEN** `SPINLOOP_ALIAS=qwen3.6-27b` is set and the user runs
  `spinloop harness apply path/to/Spinloop`
- **THEN** the argument's Spinloop is applied and the variable is ignored

#### Scenario: The variable is not shadowed by a file

- **WHEN** `SPINLOOP_ALIAS=qwen3.6-27b` is set and a file named `qwen3.6-27b`
  exists in the working directory
- **THEN** the registered Spinloop is used and no shadowing note is printed

#### Scenario: Unregistered value

- **WHEN** `SPINLOOP_ALIAS` names something that is not in the registry
- **THEN** the command fails saying `SPINLOOP_ALIAS` names an unregistered alias
  and pointing at `spinloop alias --list`

#### Scenario: Dangling value

- **WHEN** `SPINLOOP_ALIAS` names a registered alias whose Spinloop has been
  deleted
- **THEN** the command fails naming the variable and suggesting the alias be
  re-pointed or dropped
