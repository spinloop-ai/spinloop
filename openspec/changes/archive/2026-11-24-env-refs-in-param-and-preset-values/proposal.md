# Proposal

## Why

Spinloop files and engine preset `.ini` files can only state literal values, so
anything that varies per machine — an HF tag, a local model directory, a cache
path — must be copy-pasted into each file. People commit Spinloops and share
presets, and the shell, Docker Compose and Terraform all let a value pull an
environment variable in; spinloop should too. Closes #283.

## What Changes

- `PARAM` values in a Spinloop and entry values in a preset file resolve
  `$NAME`, `${NAME}` and `${NAME:-default}` references against the process
  environment (unset or empty expands to the empty string, or to the declared
  default); `$$` renders a literal `$`.
- Resolution happens where a consuming command builds its configuration —
  `spinloop serve`, `spinloop cloud deploy`, a node wake — so a printed or
  dry-run command shows the resolved value. Parsing and exporting a Spinloop
  keep the raw text, so a resolved secret never leaks through a re-rendered
  file.
- A malformed reference (an unterminated `${`, an invalid name, or any other
  `${...}` form) fails the consuming command naming the preset line or PARAM
  key, rather than passing raw text to the engine.
- Keys are never expanded: preset entry keys and PARAM keys are flag names and
  pass through as written.

## Capabilities

### New Capabilities

<!-- none: the behaviour lands as requirements on existing capabilities -->

### Modified Capabilities

- `spinloop-files`: new requirement — environment-reference syntax for PARAM
  values and preset entry values, with parsing/exporting keeping the raw text
  and resolution deferred to the consuming command.
- `local-serving`: new requirement — `spinloop serve` (including `--dry-run`)
  builds the engine command from resolved preset and PARAM values.
- `remote-endpoint`: new requirement — `cloud deploy` and a node wake derive
  the deployment from resolved preset and PARAM values.

## Impact

- New leaf package `internal/envref` (stdlib only) with the expansion.
- `internal/preset`: `Parse` expands entry values, errors naming the line.
- `cmd/spinloop`: `selectionParams` (the one PARAM→layer choke point, used by
  `buildServeArgv` and `deployConfig`) expands PARAM values and returns errors.
- Specs as listed; docs updated (Spinloop file format and preset pages).
- Values containing a literal `$` keep working: a `$` that does not start a
  defined reference stays literal, and `$$` escapes. A value that *does* spell
  `$NAME`/`${NAME}` now interpolates — the conventional, intended behaviour.
