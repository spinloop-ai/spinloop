# Delta: alias-registry

## MODIFIED Requirements

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

- **WHEN** `SPINLOOP_ALIAS=qwen3.6-27b` is set and the user runs `spinloop apply`
  in a directory with no `Spinloop`
- **THEN** the registered Spinloop is applied and a note on stderr names the
  variable, the alias and the resolved path

#### Scenario: An argument wins

- **WHEN** `SPINLOOP_ALIAS=qwen3.6-27b` is set and the user runs
  `spinloop apply path/to/Spinloop`
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
