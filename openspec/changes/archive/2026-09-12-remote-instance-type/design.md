## Context

Today the instance type a remote environment launches as is a single value for
the whole control plane: a CDK-stack setting (`remote/lib/config.ts`, default
`g6e.xlarge`) handed to the start Lambda as the `INSTANCE_TYPE` env var, used
verbatim in its `runInstance` call. The only way to change it is to redeploy
the whole control plane, which changes every environment at once.

The per-environment record that already varies between environments is the
`DeployConfig`: a JSON document stored in an SSM parameter, written by the
deploy Lambda and read by the start Lambda on every wake. It already carries
the model, runner, context, and the spinloop version pin. Adding the instance
type there reuses the existing vehicle rather than inventing a new one.

See proposal.md for motivation and the specs for the requirements this
implements.

## Goals / Non-Goals

**Goals:**
- Let one environment launch as a different instance type than the rest of the
  account, set at deploy time and read back on every fresh launch.
- Keep a fleet file and a standalone `remote deploy` of the same source in
  agreement about what a node's environment launches as.
- Stay backward compatible: an environment (or a control plane) that carries no
  type behaves exactly as it does now.

**Non-Goals:**
- No Spinloop-file keyword for the type — it stays a machine/environment-local
  setting, like the harness and an alias.
- No `remote start` flag. The type is a property of the deployment, not of a
  launch.
- No way to resize a running or stopped instance (EC2 does not allow it); a
  changed type applies on the next fresh launch.
- No per-AZ or per-request type choice.

## Decisions

### The type lives in the per-environment `DeployConfig`, not on the start request

`DeployConfig` (Go `internal/remote/remote.go`, TS
`remote/lambda/shared/deploy-config.ts`) gains an optional `instanceType`.
The deploy path records it; the start Lambda reads it on every wake and uses it
in the fresh-launch `runInstance` call, falling back to the `INSTANCE_TYPE`
env var when it is absent.

This matches how the spinloop version pin already works (a deploy-time value,
stored in the same config, applied on the next fresh boot) and keeps the start
request unchanged. A re-wake of a stopped instance never relaunches, so it
naturally keeps the type it was launched with — no extra code is needed for the
fresh-launch-only semantics; the spec states it.

*Alternative considered:* pass the type on each start request. Rejected — the
type is a property of the environment (the operator sets it once and expects
every launch to honour it), a start is not a natural place for it, and a re-wake
could not apply it anyway.

### Fallback to the stack default keeps everything backward compatible

Absent `instanceType` → the start Lambda uses the `INSTANCE_TYPE` env var
exactly as today. A control plane whose `parseDeployConfig` predates the field
ignores an unknown key in the stored config, so it keeps launching on its stack
default. Per-environment types therefore require a control plane deployed with
the updated `remote/` sources; until then the feature degrades to the default
rather than failing.

### Validation in two places, one pattern

The value is interpolated into an EC2 `RunInstances` call, so a malformed value
is best named early. It is checked in the Go CLI (so a `--instance-type` typo or
a bad fleet-file value is reported before anything is sent) and in the TS
`parseDeployConfig` (the server-side authority). Both use the same shape:
`^[a-z0-9]+(?:-[a-z0-9]+)*\.[a-z0-9]+$` — a lowercase family (which may be
hyphenated, as in `u7i-6tb` and `mac2-m2`) and a size separated by a single dot.
Permissive enough for every current EC2 family, strict enough to reject obvious
junk.

The Go pattern lives in `internal/remote` (an exported validator/pattern) so
both `cmd/spinloop` (the flag) and `internal/fleet` (the file field) reuse it —
`internal/fleet` already imports `internal/remote`.

### The fleet file field is remote-only and parsed eagerly

`NodeConfig` (internal/fleet) gains `InstanceType` (`yaml:"instance-type"`).
`validate` rejects it on a `kind: daemon` node (a daemon's hardware is not
something the fleet file provisions) and checks its shape on a `kind: remote`
node, so a typo fails at file load with the node named — the file's existing
discipline. `fleet deploy` threads the node's value into the `DeployConfig` it
derives (the same seam that already carries the spinloop version), with no
fleet-level flag: the type is per-node, and a `--all` run has no single value to
apply.

## Risks / Trade-offs

- [A changed type does not reach a live instance] → The instance must be
  terminated (`remote stop` or the idle sweep) and relaunched for the new type
  to apply; a re-wake or `remote restart` keeps the old type. Documented in the
  spec and the docs; there is no EC2 API to resize, so this is inherent.
- [An old control plane silently ignores the type] → Deploys still succeed and
  launch on the stack default. Mitigation: the docs state that per-environment
  types need a re-bootstrapped control plane; the fallback means nothing breaks.
- [A valid-format type has no capacity or no quota in the region] → The
  existing per-AZ fallback and quota handling cover it; the no-capacity answer
  now names the type it was trying, so the operator sees which type was short.
- [The pattern is wrong (too strict rejects a real type, too loose admits
  junk)] → Chosen to cover all current EC2 families including hyphenated ones;
  the server-side check is the final guard even if the CLI pattern drifts.

## Migration Plan

- The Go and TypeScript changes ship in this repository. The Lambdas take effect
  when the operator re-runs `spinloop remote bootstrap` (which drives the
  updated `remote/` CDK sources); no SSM data migration is needed because the
  deploy-config parameter simply gains an optional key.
- Until a control plane is re-bootstrapped it ignores the key and launches on
  its stack default — existing fleets and environments are unaffected.
- Rollback: revert the source. A stored deploy-config carrying `instanceType` is
  harmless to an older Lambda (unknown keys are ignored), so no cleanup is
  required.

## Open Questions

None. (Surfacing the *configured* type alongside the *actual* EC2 type in
`remote status`/`metrics` is a possible later nicety; the actual type is already
reported from EC2 today, so it is out of scope here.)
