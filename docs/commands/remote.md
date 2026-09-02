# spinloop remote

Run a model too big for your laptop on a GPU in the cloud, from the same
[`Spinloop` file](../spinloop-file.md) you'd use locally — and only pay for it
while you're using it.

```sh
spinloop remote bootstrap  # once per account: deploy the control plane
spinloop remote bake       # bake the runner AMI(s) an environment runs from
spinloop remote deploy     # create an endpoint (environment) and tell it what to serve
spinloop remote start      # boot it; prints the exports your agent needs (progress on stderr)
spinloop remote status     # is it up? is it healthy?
spinloop remote logs       # what did it say? (readable after it's gone)
spinloop remote pause      # stop it now; a later start re-wakes it
spinloop remote restart    # fresh engine, same address: stop it and wake it again
spinloop remote keep 4h    # prevent the idle sweep from stopping it for 4 hours
spinloop remote stop       # terminate it now, rather than waiting for the idle timer
```

The endpoint is the one [`remote/`](../../remote/) in this repository deploys: a
GPU instance that exists only while you're using it. When it goes idle it is
stopped (so a re-wake is fast), and terminated once it has been stopped long
enough that the pause is over.

## Bootstrapping the account

Before any endpoint can run, the account-level control plane has to
exist — much like `cdk bootstrap`. `spinloop remote bootstrap` does it once per
account: it downloads the `remote/` CDK project (version-matched to your binary)
and deploys the control plane — the EC2 Image Builder pipelines, the lifecycle
Lambdas, and the shared weights bucket, roles and VPC — publishing them as
CloudFormation outputs that `spinloop remote deploy` discovers later. It bakes
**no** AMIs — that is the separate `spinloop remote bake` step below.

```sh
spinloop remote bootstrap                 # shows a consent plan, then deploys
spinloop remote bootstrap --dry-run       # print the plan and do nothing
spinloop remote bootstrap --package-manager npm  # use npm instead of pnpm
```

Before deploying, bootstrap prints a plan — the target account and region, the
control-plane resources, the cost, and the exact commands — and asks you to confirm
(`--yes` skips the prompt). It creates **no** Elastic IP or instance and **no**
environment; those come from `spinloop remote deploy`. Re-running is safe: it
updates the control-plane stack and doesn't touch any live instance. It needs Node 22, a
Node package manager, AWS credentials, and enough GPU vCPU quota for a later
launch.

By default bootstrap uses `pnpm` and falls back to `npm` when `pnpm` isn't on the
path, logging which one it picked. To pin the choice, pass `--package-manager`
(`pnpm` or `npm`) or set `SPINLOOP_REMOTE_PACKAGE_MANAGER`; the flag wins over the
env var. A pinned manager that isn't installed fails the preflight rather than
falling back. `spinloop remote bake` honours the same flags.

## Baking the AMIs

Each engine runs from a baked AMI (driver + engine, no model).
`spinloop remote bake` starts a bake for each runner you name — both `llamacpp`
and `vllm` when you name none — and **waits** until the AMI(s) are available, so
the command returns at the point `spinloop remote deploy` can go:

```sh
spinloop remote bake                # bake both engines' AMIs; waits (~20-40 min)
spinloop remote bake llamacpp       # bake one engine's AMI
spinloop remote bake --no-wait      # return once the bakes are queued
```

Bakes are slow (a builder instance runs for 20–40 minutes) and independent of
the weight seed, so `--no-wait` lets them run in parallel — the command prints
how to check on them. A bake deploys nothing: it needs the control plane's
Image Builder pipelines, so if the control plane isn't deployed it fails telling
you to run `spinloop remote bootstrap` first. Re-bake only when the engine
version or the driver changes; the model is **not** baked in, and a new AMI is
picked up automatically once it is available.

## The usual flow

```sh
eval "$(spinloop remote start)"           # boots it (~10 min from cold) and sets
                                       # OPENAI_BASE_URL and OPENAI_API_KEY
spinloop apply                           # point your agent at it
spinloop harness                         # work
spinloop remote pause                    # done for now: stopped, re-wakeable with start
spinloop remote stop                     # done for good: terminate it
```

Forgetting `stop` is not a disaster — after a spell with no requests the
endpoint is **stopped** (no more GPU billing; a start re-wakes it) and then
**terminated** once the retention passes — but it's the difference between
minutes and hours of GPU time.

## Pointing at your endpoint

`spinloop remote` needs the endpoint's control URLs, which its deployment prints.
Put them in a JSON file:

```json
{
  "start_url": "https://....lambda-url.us-east-1.on.aws/",
  "stop_url": "https://....lambda-url.us-east-1.on.aws/",
  "deploy_url": "https://....lambda-url.us-east-1.on.aws/",
  "region": "us-east-1",
  "base_url": "http://198.51.100.7:8000/v1"
}
```

`base_url` is the endpoint's own address, and it's optional — `remote` doesn't
need it, since `start` and `status` report the address themselves. It's there
for [`spinloop apply`](apply.md): a Spinloop for a remote endpoint can leave out
`BASEURL` and let apply take the address from here, so the address stays with
the deployment that owns it. A `BASEURL` in the Spinloop wins if you set one.

Either name it from the Spinloop, so a project carries its own endpoint:

```dockerfile
REMOTE ./remote.json                       # a path: resolved next to the Spinloop, like PRESET
REMOTE https://example.com/team/remote.json  # a URL, fetched instead of read from disk
REMOTE qwen3.6-27b-prod                    # a bare name: an environment in the registry
```

A relative path resolves against the Spinloop's own URL too, when the Spinloop
itself was fetched from one — and is fetched only when a command here actually
needs it, never merely because the Spinloop was read. A bare name (no slash, no
`.json`) selects a **named environment** from the
per-user registry at `~/.config/spinloop/remotes/<name>/remote.json`. This keeps
deployment state per-user and per-instance: two projects name two environments
without clobbering, and only the name — not the URLs — lives in the committed
Spinloop. `spinloop remote deploy` registers an environment for you; you can also
create one by hand.

Given no path, `spinloop remote` reads the Spinloop `SPINLOOP_ALIAS` names, or
`./Spinloop` when you are standing beside one. With neither — or with one that
names no `REMOTE` — it uses the `default` environment
(`~/.config/spinloop/remotes/default/remote.json`), so it works from anywhere.
An existing `~/.config/spinloop/remote.json` from before the registry is still
read as the default; move it to `remotes/default/remote.json` when convenient.

## Listing environments

```sh
spinloop remote ls
```

lists each registered environment with its base URL and region, marking any
whose `remote.json` is missing or unreadable. It contacts no endpoint.

Requests are signed with **your** AWS credentials (the usual profile, SSO
session, or environment variables), and the endpoint's URLs require it. Spinloop
stores no credentials of its own. Beyond invoking those URLs, the only extra
permission it wants is for [reading logs](#reading-the-logs), which talks to
CloudWatch rather than to an endpoint.

## Checking on an endpoint

```sh
spinloop remote status                   # is it up, is it healthy, where is it
spinloop remote metrics                  # what is it doing — tokens, GPU, CPU, RAM
spinloop remote metrics -w               # the same, redrawn every 60 seconds
```

Both report **`last active`** — how long since the endpoint's engine last did
any work. It comes from the activity the on-instance daemon tracks, so it is
one answer decided on the box rather than something each command re-derives
from raw counters. `status` asks the daemon alongside the health check it
already makes, so it is no slower than before.

The figure is left out rather than guessed at whenever there is nothing to
report: an engine that has not yet served anything, a daemon that cannot be
reached, and — on the cloud — an instance that is **stopped**, since reaching
the daemon needs a running box. (That last one differs from a stopped *engine*
on a machine you run yourself, which does still report; see
[`spinloop fleet`](fleet.md).) In none of these cases does the rest of the
report change.

It appears once the control plane has been redeployed with `pnpm run deploy`
(the `run` matters — plain `pnpm deploy` is pnpm's own built-in command). An
older control plane simply omits it, and the commands print what they always
did.

## Keeping an instance alive

```sh
spinloop remote keep 4h                          # retain for 4 hours from now
spinloop remote start --keep 2h                  # start and retain for 2 hours
```

`keep` sets the `Retain-Until` tag on the environment's instance, preventing
the idle sweep from stopping or terminating it before the deadline. It is a
minimum runtime — once the deadline passes, normal idle checking resumes. A
manual `pause` or `stop` still takes effect: the tag guards against accidental
death, not deliberate shutdown.

`start --keep DURATION` sets the same tag at wake time, so the instance is
retained from the moment it boots. Useful when you know you need the instance
for a fixed period (e.g. overnight debugging) and don't want to type `keep`
afterwards.

The deadline appears in `status` output when the tag is present, so you can
see how long the instance is protected for.

It requires a control plane with the update Lambda (bootstrap with a recent
version, or re-bootstrap).

## Restarting the engine

```sh
spinloop remote restart             # fresh engine, same endpoint
spinloop remote restart --force     # skip the graceful engine stop
```

`restart` stops the instance the way `pause` does — without terminating it, so
the boot disk and its weights survive — and then wakes it, blocking until the
model serves again. Because the box is only stopped, the address does not
change and the re-wake loads the weights already on disk: the fastest way back
to a fresh engine. Nothing about what the endpoint serves changes; that is
[deploy](#creating-an-endpoint-deploy)'s job.

The stop asks the on-instance daemon to shut the engine down politely first.
When the engine or its daemon is wedged and will not answer, `--force` skips
that step and takes the box down directly — the EC2 stop does not go through
the daemon, so it still lands. It also kills whatever the engine is doing,
which is why the default stays polite.

With the instance already stopped, `restart` just wakes it — the same as
`start`. Like `start`, it takes a `--timeout` (default 15m) and prints the
endpoint's base URL when it serves, so you can check the address is unchanged.

## Reading the logs

```sh
spinloop remote logs                      # the last hour of engine output
spinloop remote logs --source boot        # the start-up log, before the engine ran
spinloop remote logs --since 6h --limit 500
spinloop remote logs -f                   # follow, until you interrupt it
```

Instances ship two logs to CloudWatch: the inference engine's own output, and
the boot log covering everything that runs before the engine starts (the
weights download, credential setup). `logs` reads them from CloudWatch with
your AWS credentials, not from the instance — so **the logs outlive the
instance**. An environment that is stopped, or whose instance terminated hours
ago, still has readable logs, which is exactly when `status` and `metrics` have
nothing left to tell you.

Use `--source boot` when a start failed and the engine log is empty: the
failure happened before the engine existed, so only the boot log saw it.
`--source all` interleaves both in time order.

Output is oldest first, one event per line with its local timestamp. When more
than one instance or both sources are in play, each line is prefixed with
`source/instance`; with a single origin that prefix is left off. `--format
json` emits the same events as an array for scripting.

| Flag | Meaning |
| ---- | ------- |
| `--source` | `engine` (default), `boot`, or `all` |
| `--since` | How far back to look, as a duration (default `1h`) |
| `--limit` | Most events to print, keeping the most recent (default 200) |
| `--instance` | Only this instance's events |
| `-f`, `--follow` | Keep printing new events until interrupted |
| `--format` | `text` (default) or `json` |

If more events match than `--limit`, the earlier ones are dropped and the count
is reported — raise `--limit` to see them.

Reading logs needs one permission beyond the usual endpoint access:
`logs:FilterLogEvents` on `/cloud-vm-llm/*`. Without it the command says so
rather than reporting an empty log.

If it reports that no log group exists, the control plane was deployed before
log shipping existed; `spinloop remote bootstrap` re-deploys it and the next
instance will ship. Logs already lost with a terminated instance can't be
recovered — only what's shipped from then on.

## Creating an endpoint: `deploy`

`spinloop remote deploy` creates an **environment** on the bootstrapped control
plane and tells it what to serve. It reads the Spinloop and its preset:
`PROVIDER` picks the engine (so the file that runs a model locally under
[`spinloop serve`](serve.md) deploys the same model remotely), and `REMOTE` names
the environment — the committed link between the Spinloop and its deployment:

```dockerfile
PROVIDER llamacpp        # the engine to run: llamacpp or vllm
ALIAS    qwen3.6-27b     # the name your agent asks for — and the name served
CONTEXT  131072
PRESET   ./preset.ini    # the model and its flags
REMOTE   qwen3.6-27b     # the environment deploy creates and registers
```

Deploy discovers the control plane from the bootstrap stack's outputs, then
provisions the environment's own Elastic IP, API key, ingress rule and state,
registers it under `~/.config/spinloop/remotes/<env>/`, and stores what to serve.
Everything the endpoint sets itself — host, port, where the weights live, the
API key, the context size, the alias — is dropped from the preset, so one
preset works both locally and remotely without edits.

Who may reach the instance is **per environment**: `--allowed-cidr` sets it,
defaulting to your public IP as a `/32` on first deploy; later deploys leave
ingress alone unless you pass it again. Deploying over an environment that is
already registered, or whose instance is live, requires `--overwrite` — a
redeploy never silently clobbers a running instance.

The environment's instances install spinloop — the daemon that hosts the
engine — at boot, and `--spinloop-version` pins the release a fresh boot
installs. Without it a boot installs the latest published release; with a
pin (`1.26.1`; a leading `v` is fine) it installs exactly that. The pin is
environment state, not engine state: it takes effect at the next boot, so a
running instance keeps the daemon it was deployed with.

Deploying doesn't start anything. If the shared bucket doesn't have those
weights yet it fetches them (about 15–20 minutes, entirely on its side) and
says so; wait for that before your first `start`, or the model won't be there.

Switching model, quantisation, or engine is an edit to those two files and one
`deploy` — no redeployment of the infrastructure. A second Spinloop naming a
different `REMOTE` gets its own environment, side by side.

```sh
spinloop remote deploy --dry-run       # see what would be sent
spinloop remote deploy path/to/Spinloop  # deploy a different one
spinloop remote deploy --overwrite     # redeploy over the existing environment
spinloop remote deploy --reseed        # re-fetch weights already in S3
```

Deploy fetches the weights only when they are not in S3 already. `--reseed`
fetches them regardless — for a repo whose files changed under the same name,
or a seed you want to run again. It starts the same ~20-minute seed instance a
first deploy does, and re-downloads the weights, so it is opt-in rather than
something to reach for by habit.

## Flags

| Flag | Meaning |
| ---- | ------- |
| `--timeout` | How long `start` or `restart` waits for the endpoint (default 15m) |
| `-F`, `--force` | `restart` only: skip the graceful engine stop and take the instance down directly |
| `--keep` | `start` only: retain the instance until `now + DURATION`, preventing the idle sweep from stopping it |
| `-n`, `--dry-run` | `deploy` only: print what would be sent, without sending it |
| `--reseed` | `deploy` only: re-fetch the weights even if they are already in S3 |
| `--spinloop-version` | `deploy` only: the spinloop release fresh boots install (default: the latest published release) |

`bootstrap` and `bake` have their own too (`--ref`, `--dir`, `--region`,
`--package-manager`, and `--no-wait` on bake) — see their sections above.
`logs` has its own set — see [reading the logs](#reading-the-logs).

## Notes

- `bootstrap` and `bake` are account-level and take no Spinloop: the control
  plane and the AMIs are shared by every environment.
- `deploy` always needs a Spinloop — it's the thing being deployed. The others
  take an optional Spinloop path, a [registered alias](alias.md), or a URL.
  Given none, they use the alias `SPINLOOP_ALIAS` names, and failing that
  `./Spinloop`, and failing that your per-user config.
- `deploy_url` is optional: a config written before `deploy` existed still
  works for `start`, `stop`, and `status`.
- Only a self-hosted engine can be deployed (`llamacpp` or `vllm`). A hosted
  provider has nothing to deploy.

## See also

- [The `Spinloop` file](../spinloop-file.md) — including `REMOTE`
- [`spinloop serve`](serve.md) — the same Spinloop, run on your own machine
- [`spinloop apply`](apply.md) — point your agent at the endpoint
