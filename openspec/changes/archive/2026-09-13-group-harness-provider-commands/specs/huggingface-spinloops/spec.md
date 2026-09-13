# Delta: huggingface-spinloops

## MODIFIED Requirements

### Requirement: Creating a Spinloop from a reference

`spinloop hf <ref>` SHALL read the named Hugging Face model, derive a provider
selection from it, and render it as a Spinloop on stdout. Rendered output SHALL
be the same canonical form `spinloop harness export` produces, so
`spinloop hf <ref> > Spinloop` yields a file every other command accepts. A
missing reference SHALL fail showing the command's usage.

What was inferred, and from what, SHALL be reported on stderr — the provider
and why, the quantisation chosen and the alternatives available, the context
window and where it came from, and whether a local copy was used — so stdout
stays a clean Spinloop while the reasoning is still visible.

#### Scenario: A reference becomes a Spinloop

- **WHEN** the user runs `spinloop hf unsloth/Qwen3.6-35B-A3B-GGUF`
- **THEN** a Spinloop naming a provider, model, alias and context is printed on
  stdout, and the reasoning is printed on stderr

#### Scenario: Redirecting produces a usable file

- **WHEN** the printed output is redirected to `./Spinloop` and
  `spinloop harness apply` is run
- **THEN** the harness is configured from it with no editing

#### Scenario: No reference

- **WHEN** the user runs `spinloop hf` with no argument
- **THEN** it fails showing how the command is called

### Requirement: Printing, writing and applying

By default the Spinloop SHALL be printed to stdout. `--output-file`/`-o` SHALL
write it to the named path instead, reporting where it went; an existing file
SHALL NOT be overwritten unless `--force` is given, so a hand-edited Spinloop
cannot be lost to a mistyped command. `--apply` SHALL additionally configure
the active harness from the selection, by the same path
`spinloop harness apply` uses and honouring `--harness`/`-H`, so one command
goes from a model page to a dressed agent.

#### Scenario: Writing to a file

- **WHEN** the user runs `spinloop hf <ref> -o ./Spinloop` and no such file exists
- **THEN** the Spinloop is written there and the path is reported

#### Scenario: An existing file is not clobbered

- **WHEN** `-o` names a file that already exists and `--force` was not given
- **THEN** nothing is written and the command fails saying `--force` overwrites

#### Scenario: Applying directly

- **WHEN** the user runs `spinloop hf <ref> --apply`
- **THEN** the active harness is configured exactly as applying the printed
  Spinloop would have configured it

#### Scenario: Applying to a named harness

- **WHEN** the user runs `spinloop hf <ref> --apply --harness pi`
- **THEN** the Pi harness is configured rather than the active default
