# Remote Endpoint Specification

## ADDED Requirements

### Requirement: Deploying resolves environment references

When `spinloop cloud deploy` or a node wake derives a deployment from a
Spinloop, every value taken from the preset and from the `PARAM` instructions
SHALL have its environment references resolved (see the Spinloop Files
specification's Environment references requirement) against the process
environment at derivation time, so the resolved value — not the raw reference —
is what reaches the stored deployment configuration and the engine's command.
Names derived from resolved values (the model, its quantisation, the served
name) SHALL be taken from the resolved form. A malformed reference SHALL fail
the derivation naming the offending preset line or PARAM key, and nothing SHALL
be sent to the control plane. A value with no references SHALL derive exactly
as before.

#### Scenario: A PARAM reference deploys resolved

- **WHEN** `HF_TAG` is set to `Q8_0` and a Spinloop states
  `PARAM hf = org/model:$HF_TAG`, and is deployed
- **THEN** the deployment names the repository `org/model` and quantisation
  `Q8_0`, and no `HF_TAG` text reaches the engine's arguments

#### Scenario: A preset reference deploys resolved

- **WHEN** `TEMP` is set to `1.0` and a preset entry states
  `temp = ${TEMP}`, and the Spinloop is deployed
- **THEN** the stored configuration's engine arguments carry `--temp 1.0`

#### Scenario: Dry run shows the resolved deployment

- **WHEN** `spinloop cloud deploy --dry-run` derives from values carrying
  references
- **THEN** the printed deployment shows every value resolved, and nothing is
  sent

#### Scenario: A malformed reference stops the deploy

- **WHEN** a value carrying a malformed reference reaches derivation
- **THEN** deploy fails naming the offending preset line or PARAM key, and
  nothing is registered or sent
