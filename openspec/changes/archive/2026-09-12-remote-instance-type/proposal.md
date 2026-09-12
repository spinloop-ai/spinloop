## Why

A remote environment's instance type is fixed for the whole control plane (one
CDK-stack value, `g6e.xlarge` by default), so every environment in an account
launches the same machine. There is no way to give one environment a bigger or
cheaper GPU — the only lever today is to redeploy the entire control plane with
a different stack value, which changes every environment at once. A fleet that
mixes a small model and a large one, or that wants to trial a cheaper type,
cannot express that.

## What Changes

- A `kind: remote` node in `fleet.yaml` MAY declare an optional `instance-type`
  naming the EC2 instance type its environment launches as. It is remote-only:
  a `kind: daemon` node naming one is a parse error, and the value is
  format-checked when the file is read.
- `spinloop remote deploy` gains an optional `--instance-type` flag. When given,
  the type is recorded in the environment's stored deploy config so its next
  fresh launch uses it; when absent (or empty), the environment keeps the
  control plane's default.
- The per-environment deploy config (the SSM state the start Lambda reads on
  every wake) gains an optional `instanceType`. The control plane validates and
  persists it, and the start Lambda launches with it when present, falling back
  to the stack-level default when absent.
- `spinloop fleet deploy` derives each `kind: remote` node's deploy config
  including that node's `instance-type`, so a fleet file and a standalone
  `remote deploy` of the same source still agree about what a node deploys.
- The deploy plan (including `--dry-run`) states the instance type an
  environment will launch as, the way it already states the spinloop version.
- A changed type takes effect on the next **fresh launch** only: EC2 cannot
  resize a running or stopped instance, so a re-wake (and `remote restart`)
  keeps the original type, and the type applies once the instance has been
  terminated (by `remote stop` or the idle sweep) and relaunched.

No Spinloop-file keyword is introduced, and `remote start` takes no
instance-type flag — the type is a property of the environment, set at deploy
time, not of a launch.

## Capabilities

### New Capabilities

<!-- None: this is a cross-cutting addition to existing capabilities. -->

### Modified Capabilities

- `environment-deployment`: the stored per-environment deploy config may carry
  an optional instance type; `remote deploy --instance-type` records it; the
  deploy plan states it.
- `remote-node`: the fleet file's `kind: remote` node entry may declare an
  optional, remote-only, format-checked `instance-type`.
- `fleet-client`: `fleet deploy` includes each remote node's `instance-type` in
  the deploy config it derives and applies.
- `endpoint-lifecycle`: the start launches with the environment's stored
  instance type when present, else the stack default; the type applies on a
  fresh launch, not a re-wake; the no-capacity answer names the actual type.

## Impact

- Go: `internal/remote` (`DeployConfig` gains `InstanceType`),
  `internal/fleet` (`NodeConfig` gains `InstanceType` + validation),
  `cmd/spinloop/remote.go` (flag, plan output, derivation), `cmd/spinloop/fleet.go`
  (fleet deploy passes the node's value).
- Control plane (`remote/`, TypeScript): `lambda/shared/deploy-config.ts`
  (parse + validate the optional field), `lambda/start/index.ts` (launch with
  the stored type, fallback to the env-var default, no-capacity wording).
- Behavioural: existing environments and a control plane that predates the field
  are unchanged (absent type falls back to the stack default). Per-environment
  types require a control plane deployed with the updated `remote/` sources.
- Docs: the `fleet.yaml` reference, the `remote deploy` reference, and the
  fleet/remote examples.
