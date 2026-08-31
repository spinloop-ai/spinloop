## MODIFIED Requirements

### Requirement: Alias resolution

Wherever a Spinloop path is accepted, an argument SHALL be looked up in the
registry only when it is name-shaped — a path-shaped argument never causes a
registry read at all, so commands keep working when spinloop's own config is
absent or unreadable. A path on disk SHALL beat a registered name of the same
spelling, and the shadowing SHALL be reported, not silent. A registered name
whose target file no longer exists SHALL fail with instructions to re-point or
drop the alias. When an alias decides the path, the command SHALL say so.

That report SHALL go to stderr. It is prose about how the command was resolved
rather than the command's result, and the same resolution serves
`spinloop remote env`, whose stdout is meant to be evaluated by a shell.

#### Scenario: Alias used from anywhere

- **WHEN** the user runs `spinloop apply qwen3.6-27b` in an unrelated directory
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

- **WHEN** a registered name points at a file that has been deleted
- **THEN** the command fails suggesting `spinloop alias -n <name> <path>` or
  `spinloop unalias <name>`
