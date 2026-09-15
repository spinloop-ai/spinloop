## Why

The work commands work the items file on the machine they run on, which only
reaches an orchestrator that shares that file. To drive an orchestrator running
elsewhere — for example from a GitHub action on a different machine — there
needs to be a client of the orchestrator's own work list API. The commands
should be that client: they name the API and present its token, and the run's
view of the items is the source of truth.

## What Changes

- **BREAKING**: `spinloop work add`, `list`, `abort` and `remove` become
  clients of the orchestrator's work list API instead of working the items file
  directly. Each takes a `--url` flag naming the API's base address, and
  presents the API's token as a bearer on every request, resolved from
  `--api-token`, else `--api-token-file`, else the `SPINLOOP_API_TOKEN`
  environment — two of the flags at once is a refusal. The `--items` flag is
  removed: the file the API works belongs to the run.
- `work add` POSTs the item's fields to the API's add path; `work remove <id>`
  calls its remove path; `work abort <id>` its abort path; `work list` reads
  its list path.
- The commands report the API's answer. Where the API refuses — an item the
  validation rejects, an id the file already carries or the state has recorded
  ended, an item that is running where a remove or abort asks for it — the
  command names the refusal the way the API states it. The commands' own file
  checks, the abort marker, and the bounded waits for a run to take a change in
  are removed: the API's calls are synchronous, and the run is the one that
  works the file.
- A command that names no `--url` fails before it calls anything, naming the
  flag.

## Capabilities

### New Capabilities

(None.)

### Modified Capabilities

- `work-commands`: the whole capability changes from working the items file
  beside an orchestrator to calling the orchestrator's work list API. Every
  requirement — the group, adding, listing, aborting, removing — is re-stated
  against the API, and the file-working and wait behaviours are dropped.

## Impact

- `cmd/spinloop/work.go` — rewritten as a client of the work list API; the
  file-loading helpers, the abort marker, and the wait loops go away.
- `cmd/spinloop/work_test.go` — rewritten to work a test server standing in
  for the API, rather than a file on disk.
- `internal/orchestrator` — unchanged. It still serves the API and works the
  file; only the CLI that calls it changes.
- Docs — `docs/commands/work.md` re-written for the API, and the
  cross-references in `docs/work-items.md` and `docs/commands/orchestrator.md`
  adjusted to point at the commands as the API's client.
- Consumers — anything that worked the file through the commands (the GitHub
  action in particular) switches to naming the orchestrator's `--url`.
