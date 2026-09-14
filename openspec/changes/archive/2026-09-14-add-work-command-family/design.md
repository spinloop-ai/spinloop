# Design: the work command family

## How a command reaches the run

The run owns the lock beside the items file (`<file>.lock`, its pid in it)
and holds it for the life of the run; a command cannot take it, and does not
try. Every command works the files directly, and the run's pass does the
rest:

- the items file: commands read and atomically rewrite it (`<file>.tmp` then
  rename, the way the run's own `writeItemsFile` does). The run re-reads the
  file on a pass where its mtime or size has changed, so an edit is seen on
  the next pass — the hand-off the run already makes.
- the state file (`<file>.state.json`): commands read it without a lock and
  without recovery. Recovery — a record left running becoming failed — is
  the run's to do on its start, and a command that performed it would clear
  the very record `work abort` is told to leave. A command that must change
  the state (remove) rewrites it atomically; where the run is running, the
  run's convergence below makes the state agree anyway.
- the logs (`<file>.logs/<id>.log`): a command removes a log the way it
  removes anything: `os.Remove`, an absent file being a no-op.

`work add` is the one command that never needs the run's cooperation: an
append to the file is picked up on the next pass, and the command's own
checks — the file's current ids, the state's ended records — are the same
checks the run's in-process `Add` makes, read from the same files.

## The abort marker

`work abort` cannot stop an agent it did not launch: the child is the run's.
The marker beside the file is the channel, the issue's first option, and the
one a shell command beside the file can always use — the work list API's
abort needs a listen address and, off loopback, a token, neither of which a
command beside the file can know.

- Form: a directory `<file>.aborts/` beside the items file, one empty file
  per id in it. A directory rather than one file: concurrent aborts of
  different ids cannot stomp one another, and creating a marker `O_EXCL`
  makes a second abort of the same id a natural no-op.
- The aborting side creates the directory on demand and writes
  `<file>.aborts/<id>`. The run lists the directory on its pass: an absent
  directory means no aborts.
- The run consumes a marker by stopping the item — the same stop the
  in-process `Abort` makes: the record and the flight gone under the lock,
  the state saved, the child's `Stop`, the grace, then `Kill` — and removes
  the marker file when the child has ended. A marker for an item that is not
  in flight is removed without further action: the aborting command checked
  the state before asking, and the run owes the marker a consumption, not a
  second refusal.
- One-pass suppression: the ids a pass took up are not admitted on that
  pass. An abort is a take-out, and re-launching the item in the same pass
  that stopped it would put it right back where the operator took it from;
  the spec's "the next pass admits it again" is the pass after the one that
  took the marker up.

## The run's convergence with the file

A record can outlive its item where an external writer removes the item from
the file: the run's pass saves its whole record map, so a record no item
carries would stand in the state forever — and a stood-ended record would
refuse the id's re-add, the file itself no longer carrying the id to make
the refusal meaningful. So each pass, under the lock, after the re-read, the
run drops a record whose id the file no longer carries and that is not in
flight. An in-flight item the file has let go keeps its record until the
agent ends — the stop is the run's alone — and the record goes on the pass
after the end.

The two convergences compose: `work remove` rewrites the file and the state
and takes the log; where the run is running, the run's next pass prunes the
record if the command's state write lost a race to one of the run's saves.
The file is the source of truth the run already treats it as, and the state
follows the file.

## Bounded waits

`work abort` and `work remove` report an outcome, not a request. Where the
lock's pid is alive, the command polls, on a 200ms cadence, for the outcome
it asked for — the marker gone, for an abort; the record gone from the state
file, for a remove — until a 30-second bound, and names the item where the
bound runs out first. Thirty seconds is well past the run's default 2-second
tick plus the stop's 5-second grace, so a timeout says the run is not
consuming, not that the work is slow. Where the lock is absent or its pid is
dead, there is nothing to wait for: abort says so and changes nothing, and
remove's own write is the final word.

## What the commands are not

- No network: a command never talks to the run's work list API. The API
  stays the path for a caller that has an address and a token; the commands
  are the path for a terminal beside the file, and the two reach the same
  state by the same hand-off, the file.
- No child bookkeeping: the run does not record a child's pid in the state.
  The state stays a record of what happened to an item, and a command beside
  the file stays able to read it without knowing how an agent is launched.
- No confirmation prompt: remove deletes a log, but the item's instructions
  sit in the file the operator edits, and the refusal for a running item is
  the guard the operation needs.

## Surface

New in `internal/orchestrator`, the package the commands already live beside:

- `LockStatus(itemsPath)` — the lock's holder and whether it is alive: the
  liveness the run's `takeLock` already checks, read-only.
- `RequestAbort(itemsPath, id)` / `AbortsPending(itemsPath)` — write and list
  the markers; the run's pass consumes them through a `WorkList` method that
  takes the lock, stops the flights, and reports the ids it took up so the
  pass can suppress them.
- `LoadStateFile(itemsPath)` — the state read with no lock and no recovery,
  an absent file being an empty record.
- A `Join(items, records)` — the file's items, in file order, each with its
  record, backlog where there is none — the view `work list` renders and the
  run's `List` already computes, factored so the command and the run word one
  fact the same way.

`cmd/spinloop/work.go` carries the group: a `work` parent with `--items` on
each subcommand, `add`'s field flags, `abort`'s and `remove`'s id argument,
and `list`'s `ls` alias. `list` renders one plain line per item — id, state,
node, started, ended, a dash where a value is absent — and where the output
is a terminal, the state column takes the state colour: green for done, red
for failed, amber for running, and backlog left in the terminal's own
foreground, the palette's state group drawn the way every other surface draws
it.
