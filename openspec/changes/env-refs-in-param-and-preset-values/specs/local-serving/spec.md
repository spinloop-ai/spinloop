# Local Serving Specification

## ADDED Requirements

### Requirement: Serving resolves environment references

When `spinloop serve` builds its engine command, every value it takes from the
referenced preset and from the Spinloop's `PARAM` instructions SHALL have its
environment references resolved (see the Spinloop Files specification's
Environment references requirement) against the process environment as serve
holds it at that moment — on the supervised and deployed paths, where the
command applies the Spinloop's `ENV` instructions and adjacent `.env` before
building, that includes the variables the Spinloop itself contributes.
Resolution happens before layer merging and rendering, so
a resolved value participates in override and canonicalisation exactly as a
literal one does. A printed command — including `--dry-run` — SHALL show the
resolved value, never the raw reference. A value with no references SHALL
render exactly as before.

#### Scenario: A preset value resolves into the command

- **WHEN** `HF_TAG` is set to `Q8_0` and a Spinloop's preset section states
  `hf = org/model:$HF_TAG`
- **THEN** the printed command carries `org/model:Q8_0`

#### Scenario: A PARAM value resolves into the command

- **WHEN** `TEMP` is set to `1.0` and the Spinloop states `PARAM temp = $TEMP`
  alongside a preset that sets `temp = 0.7`
- **THEN** the printed command carries a single temperature flag with value
  `1.0` — the resolved PARAM standing over the preset as an unresolved one
  would

#### Scenario: The Spinloop's own ENV feeds a PARAM value

- **WHEN** a Spinloop states `ENV SLOT=3` and `PARAM port = ${SLOT}000`, and
  the command applies the Spinloop's environment before building
- **THEN** the printed command carries `--port 3000`

#### Scenario: Dry run shows the resolved command

- **WHEN** `spinloop serve --dry-run` runs against a Spinloop whose values
  carry references
- **THEN** the printed command shows every value resolved, and nothing starts

#### Scenario: Resolution failure fails the command

- **WHEN** a value carrying a malformed reference reaches command building
- **THEN** serve fails naming the offending preset line or PARAM key, and no
  engine starts
