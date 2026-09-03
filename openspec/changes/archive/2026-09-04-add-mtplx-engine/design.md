# Design

## The engine table entry

`engineFor` gains a `mtplx` case returning the MTPLX serve params:

| Spinloop field | flag                  | notes                                                        |
| -------------- | --------------------- | ------------------------------------------------------------ |
| MODEL          | `--model`             | an HF repo id or a local path, verbatim                      |
| ALIAS          | `--model-id`          | omitted when unset; the engine derives a name otherwise      |
| CONTEXT        | `--context-window`    | a token count, parsed as today                               |
| PARALLEL       | `--max-active-requests` | an admission cap; see the Parallelism delta               |
| BASEURL        | `--host` / `--port`   | omitted entirely when unset, so MTPLX's own defaults stand   |

- `needsModel` is true. spinloop forbids a silent default model, and MTPLX
  has one; the Spinloop names what it serves.
- `--download` is always passed. MTPLX's auto-fetch applies to its own
  optimised packs and is a no-op when the model is already present, which is
  the closest analogue to `llama-server -hf`. Other weight formats are
  obtained with `mtplx pull`, which stays outside this capability.
- A key, when set, renders as `--api-key-file <path>`: the daemon writes the
  key to a `0600` file and the secret never appears on the command line, the
  same discipline as every other engine.
- The binary is expected on the `PATH`; the brew, pip, and app installs all
  put it there, so there is no conventional install location to fall back to.
- The preset dialect is the zero value: every MTPLX preset key is a long-form
  flag, so keys pass through unchanged and the override-by-canonical-name
  rule needs no alias table.

## Readiness

`readinessCheckedRunners` gains `mtplx`. MTPLX's `/health` answers
`200 {"ok":true}` once its OpenAI server is up, which may be before the
weights finish loading. That is fine: routing treats `running` as "a process
exists" and launches only once the engine endpoint actually answers, so a
still-loading node is waited for — the existing "A started engine that is not
yet loaded is waited for" behaviour, unchanged.

The map alone is not enough, because the readiness check needs the engine's
*address*, and that was only being carried on the metrics scrape target —
which mtplx has none of, since it has no /metrics dialect. So the address is
decoupled from the dialect:

- `scrapeTargetFor` always resolves the address (the engine's own `--host`/
  `--port`, else BASEURL, else the engine default) even when the engine has no
  metrics dialect; the target's dialect field stays empty in that case.
- The activity sampler skips a target that has an address but no dialect —
  there is no /metrics to parse, so an engine that cannot be scraped still
  costs nothing.
- The readiness check is keyed on the **runner**, not the dialect. That is
  what the daemon-api spec's "the runner has a known health-check convention"
  actually means, and it is what lets a dialect-less engine like mtplx be
  probed at `/health` on its own address. omlx stays unchecked: it is not in
  the map, so its readiness field remains absent.

## Metrics: none

MTPLX's `/metrics` is a JSON ring of per-request envelopes, not a Prometheus
text endpoint with cumulative counters. There is no dialect to write, so
`mtplx` gets the omlx treatment: no scrape target, no token stats, the
activity sampler samples nothing, and a fleet node's "last active" figure is
omitted rather than implied. If MTPLX ever grows cumulative counters, that is
a separate change with its own dialect.

## The runner-aware Spinloop-to-deploy-config path

`deployConfig` (cmd/spinloop/remote.go) is shared by the cloud and node
paths, and today hardcodes llama.cpp-shaped assumptions. The change makes the
node path runner-aware and leaves the cloud path byte-for-byte as it is:

- **The gate moves to the target.** The cloud keeps its runners — `llamacpp`
  and `vllm` — and its error message. The node path accepts `llamacpp`,
  `vllm`, and `mtplx`. MTPLX is Apple-Silicon-only and has no machine image,
  so it never becomes a cloud runner. Every consumer of the node path — a
  routed wake, `spinloop fleet start` (which the fleet-client spec says uses
  the same node-owned derivation), and the `fleet route` dry-run — gains
  mtplx together, so no caller needs its own change.
- **Preset fallback keys become per-runner.** The model falls back to the
  preset's `hf` key for `llamacpp` and `vllm` (unchanged) and to `model` for
  `mtplx`. The context falls back to `ctx-size` for `llamacpp` and `vllm`
  (unchanged) and to `context-window` for `mtplx`.
- **The local-path check follows the weights.** A local model path is
  refused only where the destination fetches the weights itself (the cloud):
  it cannot ship a file. A node has the file the Spinloop points at, so the
  node path carries the path as the model to load. Today the check is
  unconditional and fires even on the node path, which blocks llamacpp and
  vllm wakes with local weights as well; moving it unblocks those too.
- **Quant splitting stays as it is.** `splitModelQuant` is a no-op for
  mtplx: an HF repo id and a local path contain no colon, and a colon, if one
  ever appeared, splits and rejoins at the same place when the daemon builds
  the command, so the reference round-trips exactly.
- **Owned preset keys.** `model` and `api-key`/`api-key-file` are already in
  the owned set. `model-id`, `context-window`, and `max-active-requests`
  join it, so a preset's raw values do not double-define the computed
  flags. `host` and `port` stay node-owned and preserved, as now.
- **`parallelPresetKey` gains its `mtplx` case** — `max-active-requests` —
  which the node path makes reachable. Its `omlx` case stays unreachable
  until #159.

The daemon side needs nothing new: `validateDeployConfig` resolves the runner
through `engineFor`, which now knows `mtplx`, and `BuildArgv` renders a
deploy config through the same engine table as `serve`.

## Non-goals

- **omlx fleet wake.** The runner-aware path makes it a small follow-up
  (#159); keeping it out keeps this change's deltas reviewable.
- **The scheduling mode as a Spinloop field.** `--scheduler-mode` (serial /
  parallel / concurrent) is per-deployment tuning that belongs in a preset,
  the same way `--speculative` and friends do elsewhere. `PARALLEL` only
  admits requests; how they execute is the preset's call.
- **The MTPLX app's own server.** If the app is already serving on the port,
  `mtplx serve` exits naming the occupant; there is no attach mode, and
  spinloop does not try to adopt a foreign process.
