# Design

## Context

Three places decide what cloud things exist today: `./fleet.yaml` (authored,
committable, already resolved from the working directory by
`internal/fleet/config.go`), `~/.config/spinloop/clouds/<name>/cloud.json`
(written by deploy, read by everything else), and `ForEnvironment`'s
in-memory fleet of one (`internal/fleet/config.go`). The proposal removes the
second and re-points the third.

Two facts make this smaller than it looks:

- `runDeploy` already resolves the plane from credentials and region before it
  needs anything else (`cmd/spinloop/cloud.go`, comment: *"the node may
  not exist yet, so there is nothing local to resolve"*), via
  `cloud.DiscoverControlPlane` and `controlPlaneFromOutputs`
  (`internal/cloud/aws.go`). The proposal removes the two local acts that
  follow — the `os.Stat` guard and `SaveEnvironment` — and reuses that path for
  every command.
- The credential already carries the plane's identity: `keyringService` keys
  stored control-plane keys by region (`internal/cloud/keystore.go`), and the
  `cloud-vm-llm-remote-cli` policy already grants `cloudformation:DescribeStacks`
  scoped to the control-plane stack (`remote/lib/llm-stack.ts`). Discovery needs
  no new grant.

The server already holds the facts the files cached: `/cloud-vm-llm/<env>/deploy-config`
(`remote/lambda/shared/environments.ts`), the node's Elastic IP by the
`cloud-vm-llm:env` tag, its key secret, and its schedules. The start Lambda
already answers `state: 'unconfigured'` when there is no deploy config, so
existence is per-name today; only enumeration is missing.

## Goals / Non-Goals

**Goals:**

- One resolution rule for a plane, used identically by every cloud call site,
  with no branch on the node's name.
- Enumeration and ownership answered through the existing control API, so no new
  IAM grant is needed by any caller.
- Every error that used to say "not registered" says what was looked for and on
  which plane.

**Non-Goals:**

- No new file format. `fleet.yaml`'s schema is unchanged; there is no
  `cloud.yaml`, no plane section, and no local cache format.
- No plane naming, plane maps, or per-node plane references. One process, one
  credential, one plane.
- No migration tooling, and no reader for superseded `clouds/` files.
- No change to the keystore, its per-region indexing, or `cloud auth`.

## Decisions

### 1. The credential and region are the plane's identity

`(credential, region)` → stack `cloud-vm-llm` → its outputs. Region comes from
the standard AWS resolution (`SPINLOOP_CLOUD_REGION`, then `AWS_REGION`, then
the profile), which is what `LoadAWSConfig(ctx, region)` already accepts.

*Rejected — a nameable plane in a file.* A committed file would have to name the
account to be unambiguous, and `scripts/check-no-cloud-identifiers.sh` matches
12-digit account ids in every tracked file. Credentials are therefore the only
lint-safe selector, which is also the idiomatic AWS answer: `AWS_PROFILE` picks
the account. If multi-plane fleets ever matter, a node naming an AWS *profile*
is the lint-safe extension, and it is deliberately not built now.

*Consequence accepted:* two bootstrapped accounts in one region collapse to the
plane the ambient credential chooses. This already happens — the keystore keys
entries by region alone and merely records the account inside — so the change
makes an existing constraint visible rather than creating it.

### 2. Resolution is remembered in memory, never on disk

One resolution per process, memoised behind the resolver, plus revalidate-once:
a call rejected as unauthorised or failing with an unreachable host triggers a
single re-resolution and one retry.

*Rejected — a TTL'd disk cache.* That is a plane file with an expiry, which is
how `cloud.json` was born; its failure mode is indistinguishable from the thing
being removed. *Rejected — never cache.* `fleet dashboard` polls for hours and
the gateway resolves per request; both would hammer CloudFormation to learn
something that changes once per re-bootstrap.

Durable state and derived state differ exactly here: a stale authoritative file
misroutes silently forever, a stale in-memory answer produces a rejected call,
which is a signal that heals itself.

### 3. Enumeration is answered by the control plane, not by AWS APIs the CLI calls

A branch on the existing control API returns the plane's nodes by reading
the `/cloud-vm-llm/` parameter path and the tagged resources with the Lambda's
own role.

*Rejected — giving the CLI `ssm:GetParametersByPath`.* It works, but it widens
the caller policy for every deployment, and it breaks the shape the CLI already
has: the control API is the only thing the CLI speaks about a node, and
the only AWS reads it makes today are stack outputs (granted for precisely this),
pricing, and its own access keys.

### 4. Deploy records `deployedBy` and `deployedAt` in the stored deploy config

The identity is the one the control plane already sees in the SigV4 signature.
Both fields ride in the inventory reply and in the deploy reply.

*Rejected — an IAM `Tag` lookup for the creator.* It costs a call per listing and
only ever reports the same principal the invocation already names.

This is what makes plane-global names tolerable: a refused deploy must be able
to say who holds the name and what they deployed, and `cloud ls` on a shared
plane is unreadable without it.

### 5. Redeploying over an existing node without a stated CIDR refuses

Existence is asked of the plane, so "is this node new?" no longer means
"does this machine have a file". A new node keeps the current default
(detected public `/32`); an existing one with no stated `--allowed-cidr` is
refused, naming the flag.

Today the local file's presence is what protects an existing node's
security group (`if allowedCidr == "" && !registered` in `runDeploy`). Left as
it was, a second engineer deploying a shared node would silently widen a
colleague's ingress to their own IP. *Rejected — leaving ingress alone when no
CIDR is stated:* it is a silent no-op for someone who meant to change it.

### 6. `cloud undeploy` is a consequence, not a feature

With no local record, an abandoned node leaves no trace while holding an
allocated Elastic IP, an open security group, a key secret and any schedules.
Undeploy takes down exactly the node-owned resources — instance, EIP,
security group, secret, schedules, deploy config — and requires a named
node plus explicit consent, mirroring the rule that no cloud subcommand
acts on an unnamed instance.

*Rejected — `cloud forget` (delete only the local record).* There is no local
record to forget.

### 7. Membership is the committed `fleet.yaml`; the plane never adds members

The plane decides existence; the file decides membership.

*Rejected — plane-listed membership* (the fleet is the file's daemons plus every
plane node): on a shared plane a gateway would route agent traffic into a
teammate's half-deployed experiment and an orchestrator would dispatch work onto
it. *Rejected — an accompanying `cloud.yaml`:* `NodeConfig` already carries the
deploy intent (`file:`, `instance-type:`), so a second file would split one
struct across two documents and force a join on a bare name — with two new ways
to disagree (a node with no declaration, a declaration nobody drives).

### 8. Migration is a no-op

Nothing reads `clouds/`, nothing is written there, and nothing is deleted on
upgrades: stale files are simply ignored, and the release note says they may go.
The `SPINLOOP_CLOUD_*` path is unchanged, so CI configured entirely by
environment variables keeps working — it is now simply the documented way to
state a plane outright rather than a fallback for a missing file.

## Risks / Trade-offs

- **[Every cloud command now depends on CloudFormation]** → One call per process
  behind the memo; the failure text names stack, account and region and the
  bootstrap fix; revalidate-once absorbs a re-bootstrap without a restart.
- **[`cloud ls` and `--node` completion need credentials and network]** → Completion
  already must never error, so it degrades to empty; `cloud ls` fails loudly
  naming where it looked, which beats today's confidently wrong empty list.
- **[Names become plane-global: a shared `dev` is one node]** → Owner and
  timestamp in the inventory and in the refusal; private nodes move to a
  personal account or get distinct names. This is the intended meaning of a
  shared plane, and the old per-user isolation was already broken by it.
- **[Undeploy destroys shared infrastructure]** → One explicitly named
  node, never a set; consent required; `--dry-run` reports the resource
  list without deleting.
- **[Two accounts, one region collapses]** → Documented; ambient credential wins,
  as it already does over the stored key; `AWS_PROFILE` is the escape hatch.
- **[Retiring the word "environment"]** → Requirement headers that carry it are
  renamed through RENAMED sections, so the archive sync lands the node names.
  Scenario titles inside MODIFIED blocks are kept byte-exact — the delta matches
  on them — and are reworded at archive, once the synced spec is theirs alone.

## Migration Plan

Land in two deploys, both green alone:

1. **Control plane first** — inventory branch, ownership fields, undeploy path.
   Older CLIs ignore all of it; nothing is removed.
2. **CLI second** — plane resolution, registry removal, `cloud ls` and
   completion, `undeploy` wiring, error text.

Rollback is a CLI downgrade: the control-plane additions are additive, and a
downgraded CLI resumes writing its own `clouds/` files. No AWS state is
transitioned, so there is nothing to unwind.

## Open Questions

- Whether the inventory reply should include a node's *suggested* wake
  cost (instance type × price) so `cloud ls` can rank shared nodes by
  expense. Answerable after the capability exists, without changing any
  requirement here.
