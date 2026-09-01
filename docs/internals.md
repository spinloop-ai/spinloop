# Implementation notes

This is maintainer reference, not a user guide (see [`docs/README.md`](README.md)
for that) and not a behavior spec (see `openspec/specs/` for that — the
requirements below are already covered there). It's the gotchas and
cross-cutting rationale worth knowing before you touch the code, that don't
fit either of those.

## Gotchas

These are mistakes already made here; each was silent rather than loud, which is what makes them worth writing down.

**The catalogue is embedded at build time.** `providers.yaml` is `//go:embed`-ed, so a previously-built `spinloop` binary keeps applying the *old* catalogue no matter what the file says. Rebuild before testing any catalogue change, or you will "verify" a fix that is not in the binary you ran. (`--providers`/`SPINLOOP_PROVIDERS` reads a file at run time and sidesteps this.)

**A preset section is not the whole preset.** `Preset.Select` returns only the named `[section]`; the `[*]` defaults live separately in `Preset.Global`. Anything that consumes a section's `Params` directly — rather than going through `Args`/`Command`, which layer both — silently drops whatever the user put in `[*]`. That is usually the settings they consider obvious enough to write once, like `ngl` and `jinja`, so the failure surfaces later as a model running on CPU or refusing tool calls.

**Match preset flags by canonical name.** Preset keys have short aliases (`ngl` is `n-gpu-layers`, `c` is `ctx-size`, `hf` is `hf-repo`, `a` is `alias`). Comparing raw keys against a list of flag names therefore matches only the spelling you happened to think of. Use `preset.CanonicalKey`, which is exported for exactly this; `Flags` already dedupes layers by canonical name, last layer winning.

**A preset dialect is not interchangeable.** `internal/preset` parses any INI the same way, so an oMLX preset fed through llama.cpp's dialect parses cleanly and produces a *wrong* command: the alias table rewrites `m` to `--model` and `c` to `--ctx-size`, and the boolean table drops a `key = 0` entirely. Nothing errors — the server just receives flags it does not accept, or silently loses a setting. The dialect always comes from the engine `PROVIDER` names, never from the file.

**A busy engine does not answer its own metrics endpoint.** llama.cpp serves `/metrics` from the same queue it serves inference from, so a scrape taken while a prompt is being processed waits for that prompt to finish — tens of seconds on a long context. `/v1/metrics` used to scrape inline, so the handler blocked for as long as the engine had work, and `spinloop fleet metrics` (5s client timeout, against a handler whose scrape timeout was also 5s) could not win that race: the view went blank exactly when there was something worth watching, reporting the node as `unreachable`. The counters now come from the background sampler's last reading (`engineSample`), which the daemon takes every 15s regardless of who is asking — so the handler never waits on the engine, and staleness is bounded by the sample interval. Three things to preserve if you touch this: the *cached scrape error* is still reported, because silently omitting the token block is what once hid a scraper pointed at the wrong port; the sample is forgotten on start, so one engine's counters are never reported against the next; and the sampler retries at `catchUpInterval` (1s) until a reading lands, dropping to the full interval only afterwards — without that, a freshly started engine reports no counters for up to 15s, which is exactly the window someone watching a node they just started is looking at.

**An exported-but-empty variable is a gap, not a choice.** `setEnvIfAbsent` keys on the variable being *present*, so an `OPENAI_BASE_URL=` in the environment counts as set and suppresses the value routing meant to supply — leaving the agent pointed at nothing. `setEnvIfBlank` is the one to use for an address or a key, matching what `harnessEnv` already does for the remote endpoint's values; the distinction only shows up when something exports an empty string, which shells do more often than you would think.

**A pre-warm that races the engine's faults cannot win.** The cloud's model load is I/O-bound and the engine arrives first: it maps its weights and faults pages in as it copies them to the GPU, so from the first second of a load the volume serves *its* per-page faults, and a provisioned gp3 root hands out at most 4,000 IOPS × ~68 KB ≈ 260 MB/s whatever else wants the disk. A sequential pre-warm reader that starts with the engine spends its first seconds ahead of the faults, then goes flat — the daemon's `read_bytes` did exactly that on a live instance (~1.5 GB of real EBS reads in the first seconds, then nothing for the whole load), because the engine's faults consume the budget and its readahead turns every later "read" into a cache hit. And the shape's 32 GB of RAM cannot hold a ~30 GB model in the page cache at all, so even a finished pre-warm would only shift the faults to a different second. Live-checked 2026-08-23: pre-warm on cost double-reads and saved no time (the ~30 GB model loaded in ~115 s either way). The feature was removed after that check; the provisioned gp3 throughput and IOPS stay, because the S3 sync is the one reader whose limit is the volume's.

**`contextsize.Parse` is decimal.** `128k` is 128000, not 131072 — a `CONTEXT` written that way is not the power-of-two window it looks like. It also *overrides* a preset's `ctx-size` (both in `serve` and in `remote deploy`), so the Spinloop, not the preset, decides the window whenever it states one.

## Dashboard (`fleet_dashboard.go` and friends)

A few Bubble Tea/lipgloss specifics that are easy to break by "simplifying":

- The `tea.Program` holds the model by **pointer**: Bubble Tea never reads a value model's `Init` back, so the first round's mutations (its generation, its deadline spend — real cloud calls, for remote environments) would be silently discarded.
- The tick reschedules itself whenever it fires; a one-shot `tea.Tick` without the reschedule leaves the board still after the second round.
- Answers from a fan-out round are tagged with the round's generation, so a superseded reply (from a slower node, or a since-closed detail view) is discarded rather than overwriting newer state.
- A grid row joins the *corresponding lines* of the tiles it places, not the tile blocks — joining whole blocks glues the second tile's top border to the first tile's bottom border and shifts its body down a line.
- A tile's content is exactly the lines `fleet metrics` bar format prints (`renderStatBars`/`renderTokenLines` are shared, not reimplemented), so the panel and `fleet metrics` can never disagree on a number.

Behavior (panel contents, refresh cadence, start/stop/abort semantics, the detail view) is specified in `openspec/specs/fleet-client/spec.md`.

## Adapter schema references

- opencode config schema: https://opencode.ai/docs/config/. The catalogue follows it: `amazon-bedrock` is the Bedrock provider id, custom providers (`ollama`, `llamacpp`, `openai-compatible`) carry an `npm` package plus `options.baseURL`. The key is written as opencode's `{env:VAR}` substitution rather than the resolved secret, so no secret lands on disk; `spinloop harness` passes the keys it can resolve to the agent it launches, which is what makes a config spinloop wrote usable without exporting anything by hand.
- Pi custom-models schema: https://github.com/earendil-works/pi (`packages/coding-agent/docs/models.md`). `api` is one of `openai-completions`/`openai-responses`/`anthropic-messages`/`google-generative-ai`; `apiKey` supports `$ENV_VAR` interpolation. Not every provider maps to Pi — those without a `pi:` block (e.g. `amazon-bedrock`) error under the pi harness.
- lucinate connections store: https://github.com/lucinate-ai/lucinate. An OpenAI-compatible connection is `{id, name, type: "openai", url, defaultModel}`; the key comes from lucinate's secrets store or, when unset, the `LUCINATE_OPENAI_API_KEY` env var (which is how spinloop configures it — no secret on disk). Only providers with a `lucinate:` marker map; the rest (e.g. `amazon-bedrock`, the Vertex providers) error under the lucinate harness.
