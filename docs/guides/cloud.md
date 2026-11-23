# Deploy to a cloud GPU

A model too big for your laptop runs on a GPU in the cloud, from the **same
`Spinloop` file** you would use locally — and the endpoint is a machine that
stops when you do. While it is idle it is *stopped* (no GPU billing; a start
re-wakes it in minutes), and it is *terminated* once it has been stopped long
enough. Forgetting to stop it is not a disaster; stopping it is still the
difference between minutes and hours of GPU time.

## The one-time setup

Two steps, per AWS account:

```sh
spinloop cloud bootstrap   # deploy the control plane (like `cdk bootstrap`)
spinloop cloud bake        # bake the engine AMIs; waits (~20-40 min)
```

`bootstrap` prints a plan — account, region, resources, cost — and confirms
before deploying. It creates no instance and no environment; those come from
`deploy`. `bake` builds one AMI per engine (driver plus engine, no model) and
waits until they are available, so `deploy` can go when it returns. Re-bake
only when an engine version or the driver changes.

Both run on the administrator's ambient AWS credentials and need Node 22 plus
a package manager. `spinloop cloud auth --store` then keeps a day-to-day
credential in this machine's OS keystore so routine commands outlive SSO
log-ins.

## Deploy a model

The Spinloop says what the environment serves; `--node` names the environment —
a shared choice on the plane, and one that stays out of the file:

```sh
spinloop cloud deploy --node qwen3.6-27b
```

Deploy reads the Spinloop and its preset — `PROVIDER` picks the engine, so the
file that runs a model locally under [`spinloop serve`](local-serving.md)
deploys the same model remotely — provisions the environment (Elastic IP, API
key, ingress) and records it, with your name and the deploy time, on the
control plane. The plane is the record of what is deployed: every machine with
credentials for it lists the same environments, and a name taken there is
taken for everyone. A redeploy over an environment the plane already holds, or
one that is running, needs `--overwrite` and a stated `--allowed-cidr` — it
never silently clobbers a running instance, and never silently widens a
colleague's ingress. A fresh environment defaults to letting only your own IP
through (`--allowed-cidr` changes that).

## The usual flow

```sh
spinloop cloud start --node qwen3.6-27b        # boots it (~10 min from cold)
eval "$(spinloop cloud start --node qwen3.6-27b --print-env)"   # ...and exports
                                               # OPENAI_BASE_URL + OPENAI_API_KEY
spinloop harness apply --node qwen3.6-27b       # point your agent at it
spinloop harness open --node qwen3.6-27b        # work
spinloop cloud pause --node qwen3.6-27b        # done for now: stopped, re-wakeable
spinloop cloud stop --node qwen3.6-27b         # done for good: terminated
spinloop cloud undeploy --node qwen3.6-27b     # and take the environment apart:
                                               # address, key, schedules, deploy
                                               # config — consent required
```

Every command that acts on an endpoint selects it with `--node <name>` — the
flag is **required**, because half of these subcommands start, stop, or
terminate a cloud instance, and an instance nobody named is not one to act on.
A command given no `--node` fails naming the flag and listing the environments
the plane holds — there is no `default` to fall back to, because an assumed
name on a shared plane could act on somebody else's instance. `start` prints progress on stderr, and the export lines
for `eval` only with `--print-env`, so a plain `start` leaves stdout empty.

Useful along the way:

```sh
spinloop cloud status --node qwen3.6-27b   # up? healthy? where? how long idle?
spinloop cloud logs --node qwen3.6-27b     # readable even after it's gone
spinloop cloud keep 4h --node qwen3.6-27b  # the idle sweep won't touch it for 4h
spinloop cloud restart --node qwen3.6-27b  # fresh engine, same address
```

Work office hours? Let the cloud start and stop it for you:

```sh
spinloop cloud schedule set --node qwen3.6-27b \
  --start "0 8 * * 1-5" --stop "0 18 * * 1-5" --timezone Europe/London
```

It's up and warm by the time you sit down, and paused when you leave. See
[`spinloop cloud`](../commands/cloud.md#starting-and-stopping-on-a-schedule).

## From another machine

An environment lives on the plane, not on the machine that deployed it: any
two machines with credentials for the plane work the same endpoint. Deploy
from one, then on the other — with **no Spinloop at all**:

```sh
spinloop harness open --node qwen3.6-27b
```

The environment's runner becomes the provider, its served model the model, its
context size the window — read live, so a later redeploy is picked up
automatically rather than re-copied by anyone.

## Where next

- [`spinloop cloud`](../commands/cloud.md) — every subcommand, in full
- [Run a fleet](fleet.md) — cloud environments as fleet nodes, alongside daemons
- [Deploying your own control plane](https://github.com/spinloop-ai/spinloop/tree/main/remote)
  — the AWS project behind the command, for the curious
