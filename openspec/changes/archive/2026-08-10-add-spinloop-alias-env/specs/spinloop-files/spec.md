## MODIFIED Requirements

### Requirement: Spinloop path resolution

Commands that take a Spinloop path (`apply`, `unapply`, `serve`, `alias`,
`harness --spinloop`, and the `remote` subcommands) SHALL default to `./Spinloop`
when no path is given, SHALL accept a directory and use the `Spinloop` file
inside it, and SHALL accept a registered alias name in place of a path. When
the default `./Spinloop` is missing, the error SHALL suggest passing a path or an
alias.

When no path is given, the `SPINLOOP_ALIAS` environment variable SHALL be
consulted before falling back to `./Spinloop`, so the resolution order is the
argument, then `SPINLOOP_ALIAS`, then `./Spinloop`. `spinloop alias` SHALL be the one
exception and always use its argument or `./Spinloop`. Where the default
`./Spinloop` is missing and no `SPINLOOP_ALIAS` is set, the error SHALL name the
variable alongside the path and alias it already suggests.

#### Scenario: Bare command in a project directory

- **WHEN** the user runs `spinloop apply` in a directory holding an `Spinloop`
- **THEN** that file is applied

#### Scenario: Directory argument

- **WHEN** the user runs `spinloop apply path/to/dir` and the directory holds an
  `Spinloop`
- **THEN** `path/to/dir/Spinloop` is applied

#### Scenario: A remote subcommand resolves the same way

- **WHEN** the user runs `spinloop remote status` in a directory holding an
  `Spinloop`
- **THEN** that Spinloop is read to find the endpoint's configuration

#### Scenario: The environment names the default Spinloop

- **WHEN** `SPINLOOP_ALIAS` names a registered alias and the user runs
  `spinloop serve` with no argument
- **THEN** that alias's Spinloop is served, whether or not the working directory
  holds one

#### Scenario: Nothing to resolve

- **WHEN** the user runs `spinloop apply` with no argument, no `SPINLOOP_ALIAS` set
  and no `./Spinloop` present
- **THEN** the command fails suggesting a path, an alias, or `SPINLOOP_ALIAS`
