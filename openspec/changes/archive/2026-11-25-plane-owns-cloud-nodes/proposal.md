# Proposal

## Why

Cloud nodes are shared infrastructure, but spinloop records them
per-user: `spinloop cloud deploy` writes a copy of the control plane's URLs
into `~/.config/spinloop/clouds/<name>/cloud.json`, and every later command
resolves a node by looking for that file. The record is therefore a
cache of facts the cloud already holds — the control plane keeps each
node's deploy config, address, key and instance — and it is a cache
that only exists on the machine that deployed. A teammate with valid
credentials is told `environment "prod" is not registered` about a
node that is live and theirs to drive, `cloud ls` reports what this
machine remembers rather than what exists, and a node created on
another machine is invisible until someone redeploys it.

## What Changes

- **BREAKING** The per-user registry is removed. spinloop holds no
  record of which cloud nodes exist; the control plane is the record.
- **BREAKING** The control plane's endpoint URLs are no longer stored anywhere.
  A command resolves the plane from its credentials and region — the stack
  `cloud-vm-llm` in the caller's region, read from its CloudFormation outputs,
  exactly as `spinloop cloud deploy` already does — and remembers the answer
  for the lifetime of the process, re-resolving once when a call is rejected
  as unauthorised or unreachable.
- A node is identified by its name on the resolved plane, and names are
  plane-global: two people sharing a plane share its names. **BREAKING** for
  anyone who relied on two accounts' same-named nodes being separate.
- The plane learns to enumerate its nodes, and each deploy records who
  deployed it and when, so a shared name is explainable: `spinloop cloud ls`
  lists the plane's nodes with their owner, what each serves and where
  it answers.
- **BREAKING** `spinloop cloud ls`, `--node` completion and node
  resolution now need working credentials and a reachable plane. Completion
  still never errors: it offers nothing rather than failing.
- New: `spinloop cloud undeploy` tears a node's resources down —
  instance, Elastic IP, security group, key secret, schedules and deploy
  config — because once nothing is local, an abandoned node has no
  local trace and keeps an address allocated.
- **BREAKING** Deploying over an existing node whose ingress was not
  stated now refuses instead of defaulting to the deployer's public address.
  The guard's "is this node new?" now comes from the plane: a
  second engineer deploying a shared node must not silently widen a
  colleague's security group.
- `kind: cloud` fleet entries keep their exact current shape, and now mean
  "the plane my credentials resolve, node `<name>`". A committed fleet
  file with cloud nodes therefore works for anyone with credentials, which is
  the first reason it has been worth committing. The fleet file is the
  membership allowlist: a plane node the file does not name is not in
  the fleet, so gateways and orchestrators never route work into a
  colleague's half-deployed experiment.
- Nothing is migrated. The plane already knows everything the files did.
  `~/.config/spinloop/` keeps only what is genuinely machine-local: stored
  credentials and the CDK source cache.

## Capabilities

### New Capabilities

- `cloud-plane-resolution`: how a command decides which control plane to
  speak to — credentials and region, the stack and its outputs, the
  in-process memo and its revalidation, the `SPINLOOP_CLOUD_*` overrides, and
  what a failure names when no plane resolves.
- `cloud-node-inventory`: the plane as the record of what exists — the
  control plane enumerating its nodes, the owner and timestamp a deploy
  records, what `cloud ls` reports, and `cloud undeploy` taking a node
  apart.

### Modified Capabilities

- `remote-environments`: the named-registry and per-user-storage requirements
  are removed; name validity, listing, node identity and "every
  node resolves by name" are rewritten to resolve against the plane.
- `environment-deployment`: deploying no longer registers anything locally;
  discovery is the only source of control URLs; the overwrite guard reads the
  plane; stated-ingress is required to redeploy over a live node.
- `remote-endpoint`: node selection and the applied base URL come from
  the resolved plane rather than from a registered `cloud.json`.
- `remote-env`: `spinloop cloud env` resolves its node through the plane.
- `remote-node`: a cloud fleet node's control configuration comes from the
  plane its credentials resolve, so a fleet file is portable.
- `fleet-config`: a `--node` target resolves against the plane; the fleet file
  is the membership allowlist; a fleet of one is a plane node, not a
  registry entry.
- `config-location`: what the config directory no longer holds.
- `cloud-command`: the storage-location requirement is replaced; `cloud ls`
  reads the plane; `cloud undeploy` joins the group.
- `shell-completion`: `--node` completion is sourced from the fleet file and
  the plane, degrading to empty rather than erroring.
- `fleet-gateway`: whether a cloud node exists is settled by resolution
  and the plane's inventory, not by a stats read that fails.

## Impact

- **CLI**: `cmd/spinloop/cloud.go` (`ls`, deploy guard, ingress default, new
  `undeploy`), `cloud_deploy`, `main.go` and `target.go` (node
  resolution and the "not registered" failures), `complete.go`.
- **internal/cloud**: `environments.go` deleted (registry IO); `cloud.go`
  (`LoadEnvironment` becomes plane resolution, `Config` loses stored URLs);
  `aws.go` (discovery memo, inventory call); new undeploy call.
- **internal/fleet**: `config.go` (`ForEnvironment` resolves against the
  plane), `node.go` and `cloud_node.go` (node construction).
- **Control plane** (`remote/`): an inventory branch on the existing control
  API, `deployedBy`/`deployedAt` in the stored deploy config, and an undeploy
  path taking down EIP, security group, secret, schedules and parameter.
  Ships first — it is additive and green on its own.
- **Tests and docs**: registry-backed tests are rewritten against a plane stub;
  `docs/` cloud pages, `docs/openapi.yaml` is unaffected; the
  `scripts/check-no-cloud-identifiers.sh` rule becomes load-bearing in the
  other direction — a committed fleet file must stay free of addresses, which
  it now always is.
- **Unaffected**: `SPINLOOP_CLOUD_*` overrides still configure a plane with
  nothing on disk; the keystore and its per-region indexing are unchanged.
