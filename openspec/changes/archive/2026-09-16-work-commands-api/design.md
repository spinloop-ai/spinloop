## Context

The work commands today work the items file and the state beside it directly
(`cmd/spinloop/work.go`), and a running orchestrator picks the change up on its
next pass. The orchestrator already serves a work list API
(`internal/orchestrator/api.go`) over the same `WorkList` the run works:
`POST /v1/items`, `DELETE /v1/items/{id}`, `POST /v1/items/{id}/abort`,
`GET /v1/items`, bearer-token auth, loopback needing no token. The API's
`Add`/`Remove`/`Abort` are synchronous — each answers once the change is done —
and they return the same refusal messages the commands use today. See
proposal.md for the motivation.

## Goals / Non-Goals

**Goals:**
- Make each work subcommand a client of the work list API: it names the API's
  address with `--url`, presents the API's token, and reports the API's answer.
- Keep the commands' surface (subcommands, the add field flags, the `ls`
  alias, the plain-line list) recognisable, so a caller switches from `--items`
  to `--url` and little else.
- Reuse the codebase's existing token resolution and the existing list
  rendering rather than inventing new ones.

**Non-Goals:**
- No change to `internal/orchestrator`: it still serves the API and works the
  file; only the CLI that calls it changes.
- No offline or file-based mode. A command that cannot reach the API fails;
  there is no fallback to working a file beside it.
- No streaming, retries, or timeout tuning beyond a single sensible request
  bound; the API is local to the orchestrator's host.

## Decisions

- **The command is a thin HTTP client.** Each subcommand builds one request to
  the API's path and reports the answer. `add` POSTs the item's fields
  (`id`, `instructions`, `dir`, `tags`, `priority`) to `/v1/items`; `remove`
  and `abort` take the id as a positional argument and call
  `DELETE /v1/items/{id}` and `POST /v1/items/{id}/abort`; `list` GETs
  `/v1/items` and renders the returned items. Alternatives considered: keeping
  the file logic and adding the API as an option (rejected — the direction is
  API-only), or a shared "work client" type the subcommands call (the shape it
  lands on, kept small: one helper that builds the request, checks the status,
  and surfaces the API's message).
- **Token resolution reuses `daemonToken`.** The commands take `--api-token`
  and `--api-token-file`, resolve through the existing `daemonToken` helper
  (flag, else file, else the `SPINLOOP_API_TOKEN` environment, two flags a
  refusal), and present the result as `Authorization: Bearer …` on every
  request. This matches the daemon, gateway, and orchestrator rather than
  introducing a new token convention.
- **`--url` is required and is the base address.** The command joins the API's
  path onto it. Where it is absent, the command fails before any request,
  naming the flag. The `--items` flag is removed: the file the API works
  belongs to the run, and naming it from a client would be a lie.
- **The API's error is the command's error.** Where the API answers a non-2xx,
  the command reads the JSON error's message and fails with it, so a refusal
  reads the way the API states it ("the items file already carries an item with
  id …", "item … is running: abort it first"). This keeps the idempotent
  clients (the GitHub action) reading the same text they do today. Alternatives
  considered: re-deriving the refusal client-side from the status code
  (rejected — it would duplicate the API's wording and drift).
- **List rendering is unchanged in shape.** `work list` decodes the API's
  `{object: list, data: [ItemView]}` and renders each `ItemView` with the
  existing `workListLine`, plain lines off-terminal and coloured on it. The
  API already returns the joined view, so the command does no file or state
  reading of its own.
- **No client-side waits.** Today's `awaitAbort` and `awaitRecordGone` loops
  exist because the command wrote the file and a separate run had to take it in.
  The API's calls are synchronous — the run does the work and answers once it is
  done — so the client just waits on the request. The `stopGrace`-bounded abort
  is the run's, inside the API call.

## Risks / Trade-offs

- [The command is now useless without a running, reachable orchestrator] →
  This is the point of the change; a command that cannot reach the API fails
  naming the address, which is the operator's cue to start or reach the run.
- [A non-loopback API needs a token, so a caller must supply one] → The token
  resolution already covers flag, file, and environment; the docs state the
  loopback-needs-none rule the API enforces.
- [The abort call blocks for the stop's grace period] → Bounded by the run's
  own grace; the request carries a timeout so a wedged run cannot hang the
  command forever.
- [Consumers break on the flag change] → It is a deliberate breaking change;
  the migration is `--items <file>` → `--url <api address>` plus the token,
  and the GitHub action is updated in the same effort.

## Migration Plan

- Land the change; the work commands require `--url` from the first build that
  carries it. There is no dual-mode window.
- Update the GitHub action (separate repo) to pass `url` and the token and to
  drop the file commit/push.
- Rollback is a revert of this change; nothing on disk or in a file format
  changes, so there is no data migration.
