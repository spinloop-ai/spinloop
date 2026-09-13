# Harness Management Specification

## Purpose

Define the harness abstraction — the coding agent `spinloop` configures — and its
runtime selection, the stored default, and the `harness` command group: the
subcommands that manage the active harness's configuration, launch it with
`spinloop harness open`, and report it with `spinloop harness show`. opencode, Pi
and lucinate are the supported harnesses.

## Requirements

### Requirement: Harness command group

`spinloop harness` SHALL be a command group that does nothing on its own: a bare
`spinloop harness`, and any `harness` invocation whose first word does not name
one of its subcommands, SHALL show the group's help or fail with the
unknown-command error naming its subcommands — it SHALL NOT launch anything.
Its subcommands SHALL be:

- `add`, `remove`, `apply`, `unapply`, `show`, `export` — manage the active
  harness's configuration, each carrying the flags, arguments, and output its
  former top-level spelling had;
- `open` — launch the active harness (see "Launching the harness"); and
- `config` — report or store the default harness (see "Stored harness
  preference").

The old top-level spellings of the six config commands SHALL NOT exist.
Invoking one SHALL fail with an error that names the command's new home —
giving `spinloop harness add` for `add`, and so on — rather than the generic
unknown-command message.

#### Scenario: A subcommand runs under harness

- **WHEN** the user runs `spinloop harness add -p ollama -m llama3.2`
- **THEN** the add command runs with the same flags, arguments, and output that
  the former top-level `add` had before the move

#### Scenario: A bare harness shows help and launches nothing

- **WHEN** the user runs `spinloop harness` with no other words
- **THEN** the group's help is shown and no harness is launched

#### Scenario: The old top-level spelling names its new home

- **WHEN** the user runs the former top-level spelling `add`
- **THEN** the command fails, saying add has moved under harness and giving
  `spinloop harness add`

### Requirement: Harness abstraction

Each supported harness SHALL own its config format end-to-end behind a common
interface: a name, the executable that launches it, its config path, and the
operations apply / remove / read-state. Adding a harness SHALL be a matter of
implementing that interface and registering it; harness-neutral work
(catalogue loading, validation, size parsing) stays outside the adapters.

#### Scenario: Unknown harness

- **WHEN** any command is given `--harness foo` and no harness of that name is
  registered
- **THEN** the command fails listing the available harnesses

### Requirement: Harness resolution precedence

Every command that touches a harness SHALL resolve the active harness with the
precedence: `--harness`/`-H` flag, then the `SPINLOOP_HARNESS` environment
variable, then the stored preference, then the default (`opencode`). Output
that names the active harness SHALL also say where the choice came from.

#### Scenario: Flag beats everything

- **WHEN** `SPINLOOP_HARNESS=pi` is set, the stored preference is `pi`, and the
  user passes `-H opencode`
- **THEN** the opencode harness is used and the source is reported as the flag

#### Scenario: Default when nothing chooses

- **WHEN** no flag, environment variable, or stored preference selects a
  harness
- **THEN** opencode is used

### Requirement: Stored harness preference

`spinloop harness config --set <name>` SHALL validate the name against the
registered harnesses and store it as the default in spinloop's own config file,
without disturbing the alias registry sharing that file. `spinloop harness
config`, with no flag — or with `--get` — SHALL print the active harness and
its source, the stored preference (or that none is set), and the available
harnesses, without launching anything.

#### Scenario: Setting the default

- **WHEN** the user runs `spinloop harness config --set pi`
- **THEN** later commands with no flag or environment override resolve to Pi,
  and the command reports where the preference is stored

#### Scenario: Setting an unknown harness

- **WHEN** the user runs `spinloop harness config --set foo`
- **THEN** the command fails listing the available harnesses

#### Scenario: Reporting the active harness

- **WHEN** the user runs `spinloop harness config` (or `spinloop harness config
  --get`)
- **THEN** the active harness and its source, the stored preference, and the
  available harnesses are printed, and nothing is launched

### Requirement: Launching the harness

`spinloop harness open` SHALL launch the active harness's executable, forwarding
stdio and any trailing arguments untouched. Only `open` launches: a bare
`spinloop harness` SHALL NOT. When the harness exits with a non-zero code,
`spinloop` SHALL exit with that same code. When the executable is not on the
PATH, the error SHALL say which binary is missing and suggest installing the
harness.

#### Scenario: Arguments forwarded verbatim

- **WHEN** the user runs `spinloop harness open run --continue`
  and no leading argument names a Spinloop
- **THEN** the harness binary is invoked with `run --continue`

#### Scenario: A bare harness does not launch

- **WHEN** the user runs `spinloop harness run --continue`
- **THEN** no harness is launched; `run` is reported as an unknown subcommand

#### Scenario: Exit code surfaces

- **WHEN** the launched harness exits with code 3
- **THEN** `spinloop` exits with code 3

### Requirement: Applying a Spinloop on launch

`spinloop harness open` SHALL be able to apply a Spinloop before launching, two
ways. The `--spinloop`/`-O` flag applies one first: given bare it means
`./Spinloop`, and a named path must be attached (`--spinloop=<path>`) because
positional arguments belong to the harness — a detached path or alias following
a bare `--spinloop` SHALL be rejected with the attached form suggested.
Independently, a *leading* positional argument that names a Spinloop (a path, a
directory holding one, or a registered alias) SHALL be applied and not
forwarded — but only when no `--spinloop` was given and flag parsing was not
ended by an explicit `--`. A `--` immediately after a consumed leading Spinloop
SHALL be dropped; any other arguments are forwarded byte-for-byte. A leading
`--` SHALL opt out entirely. An unreadable alias registry SHALL demote the
leading argument to a forwarded one, never block the launch.

#### Scenario: Leading alias configures then launches

- **WHEN** the user runs `spinloop harness open qwen3.6-27b -- --agent-flag`
- **THEN** the aliased Spinloop is applied first and the harness is launched
  with only `--agent-flag`

#### Scenario: Bare flag applies the default Spinloop

- **WHEN** the user runs `spinloop harness open -O` in a directory holding a
  `Spinloop`
- **THEN** `./Spinloop` is applied and the harness launches

#### Scenario: Detached flag value is caught

- **WHEN** the user runs `spinloop harness open --spinloop ./dev/Spinloop`
- **THEN** the command fails telling them to write `--spinloop=./dev/Spinloop`

#### Scenario: Explicit opt-out

- **WHEN** the user runs `spinloop harness open -- qwen3.6-27b`
- **THEN** nothing is applied and `qwen3.6-27b` is forwarded to the harness

### Requirement: Showing the configured state

`spinloop harness show` SHALL report the active harness (and the source of that
choice), its config file path, the default model when the harness has one, and
each configured provider with its base URL and models — including each model's
context and output limits when set — followed by the registered aliases. It
SHALL honour the same `--harness`/`-H` override as every other command without
changing the stored default. An empty config SHALL be reported with a pointer
to `spinloop harness add`, not an error.

#### Scenario: Inspecting another harness

- **WHEN** the user runs `spinloop harness show --harness pi` while the stored
  default is opencode
- **THEN** Pi's configured providers are shown and the stored default is
  unchanged

#### Scenario: Nothing configured yet

- **WHEN** the harness config holds no providers
- **THEN** show prints the harness, config path, and a hint to run
  `spinloop harness add`

### Requirement: Keys reach the launched agent

When spinloop launches a harness, the launched agent's environment SHALL carry the
active Spinloop's local environment: the whole `.env` file beside that Spinloop, and
the Spinloop's own `ENV` instructions, in addition to the API key variables spinloop
can resolve for the catalogue's providers. The precedence, highest to lowest,
SHALL be the Spinloop's `ENV` instructions, then a variable already present in
spinloop's own environment, then the adjacent `.env`. An `ENV` instruction SHALL
therefore override an exported variable; the `.env` SHALL only fill a variable
that is otherwise unset. These values SHALL be placed only in the launched
agent's environment — spinloop SHALL NOT mutate its own process environment on this
path. When spinloop launches with no Spinloop active, the whole-`.env` overlay and the
`ENV` instructions SHALL NOT be applied, though spinloop SHALL still forward the
provider keys it can resolve. Neither harness stores a secret itself — each
resolves a reference when it runs — so a key kept where only spinloop reads it still
reaches the agent. Failure to read the provider catalogue SHALL NOT prevent the
launch.

#### Scenario: A key only spinloop can see still reaches the agent

- **WHEN** spinloop can resolve a provider's key variable but it is absent from
  the environment, and the harness is launched
- **THEN** the launched agent's environment carries that variable

#### Scenario: An explicit setting is not overridden by the .env

- **WHEN** a variable is set both in spinloop's environment and in the `.env`
  beside the active Spinloop, and the harness is launched
- **THEN** the launched agent sees the environment's value, not the `.env` value

#### Scenario: The adjacent .env fills a gap for the agent

- **WHEN** a variable is set in the `.env` beside the active Spinloop and is unset in
  spinloop's environment, and the harness is launched
- **THEN** the launched agent's environment carries the `.env` value

#### Scenario: An ENV instruction overrides both

- **WHEN** the active Spinloop sets a variable with an `ENV` instruction and the same
  variable is also present in spinloop's environment and/or the adjacent `.env`,
  and the harness is launched
- **THEN** the launched agent sees the `ENV` value

#### Scenario: Launching without a Spinloop applies no overlay

- **WHEN** the harness is launched with no Spinloop active
- **THEN** spinloop applies no whole-`.env` overlay and no `ENV` instructions; the
  agent runs with spinloop's environment plus any provider key spinloop resolves

#### Scenario: An unreadable catalogue still launches the agent

- **WHEN** the provider catalogue cannot be loaded
- **THEN** the harness is launched anyway, with the environment otherwise
  unchanged

### Requirement: lucinate is a registered harness

lucinate SHALL be a registered harness alongside opencode and Pi, selectable by
the same runtime precedence (`--harness`/`-H` flag, then `SPINLOOP_HARNESS`, then
the stored preference), and settable as the stored default. Registering it SHALL
NOT change the default harness (`opencode`) used when nothing selects one, and
SHALL NOT change how opencode or Pi behave.

#### Scenario: lucinate is available

- **WHEN** the user lists or selects harnesses
- **THEN** lucinate appears among the available harnesses and `-H lucinate`
  resolves to it

#### Scenario: lucinate can be the stored default

- **WHEN** the user sets lucinate as the stored default harness and then runs a
  command with no `-H` flag and no `SPINLOOP_HARNESS`
- **THEN** the lucinate harness is used and the source is reported as the stored
  preference

### Requirement: lucinate receives its OpenAI key at launch

When spinloop launches the lucinate harness, it SHALL inject the active provider's
resolved API key into the launched agent's environment as
`LUCINATE_OPENAI_API_KEY`, in addition to the provider key variables it already
forwards. This is what lets lucinate authenticate an OpenAI-compatible
connection whose stored secret spinloop deliberately left unwritten. As with the
other harnesses, spinloop SHALL place this value only in the launched agent's
environment and SHALL NOT write the secret to disk. When spinloop cannot resolve a
key for the active provider, it SHALL inject nothing under this name and leave
lucinate to fall back to its own stored secret or auth prompt.

#### Scenario: The active provider's key reaches lucinate

- **WHEN** spinloop launches lucinate for a provider whose key it can resolve
- **THEN** the launched agent's environment carries `LUCINATE_OPENAI_API_KEY` set
  to that key

#### Scenario: No resolvable key injects nothing

- **WHEN** spinloop launches lucinate and cannot resolve a key for the active
  provider
- **THEN** no `LUCINATE_OPENAI_API_KEY` is injected and the launch still proceeds
