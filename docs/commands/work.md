# spinloop work

Work the [work items file](../work-items.md) — the backlog the
[orchestrator](orchestrator.md) works — and the state it keeps beside it, from
the shell: add an item, read the backlog's state, stop a running item, remove
an item.

```sh
spinloop work add --id fix-parser --instructions "fix the failing tests" --dir ./parser
spinloop work list
spinloop work abort fix-parser
spinloop work remove docs-refresh
```

Each subcommand takes `--items`, defaulting to `./work.yaml` in the working
directory — the same file the orchestrator reads. The commands work the file
and the state beside it, and they work whether or not the orchestrator is
running: a running orchestrator sees the change on its next pass, and the file
beside it is the whole hand-off. Where the [work list
API](orchestrator.md#the-work-list-api) is the hand-off for a client, this is
the hand-off for a person and a script.

## Adding an item

```sh
spinloop work add --items ./work.yaml --id fix-parser \
  --instructions "fix the failing tests" --dir ./parser \
  --tag gpu=a100 --priority 10
```

The item's fields come from the flags, with the validation the orchestrator
applies — the instructions and working directory present, the tags well-formed
`key=value` pairs, no key named twice — and the file is a valid items file
after the append. An id the file already carries is refused, naming it, and an
id the state beside the file has recorded `done` or `failed` is refused too,
naming the record: an ended item is not worked again under its own id. A
missing file is created, carrying the one item, and a running orchestrator
picks the item up on its next pass.

## Listing the work

```sh
spinloop work list --items ./work.yaml
# a           backlog   -       -               -
# b           running   n       2026-09-14T10:00:00Z  -
# c           done      n       2026-09-14T09:00:00Z  2026-09-14T09:30:00Z
# d           failed    -       -               2026-09-14T08:00:00Z
```

Every item in the file with its record from the state, in the file's order:
the id, its state — `backlog`, `running`, `done` or `failed` — the node a
running item is on, and when it started and ended. One plain line per item, a
dash where a value is absent, so a program can split the columns. An item with
no record is `backlog`, and no state file at all — the orchestrator has never
run the file — means everything is backlog. The list reads the file and the
state and takes no lock, so it works whether or not the orchestrator is
running; the state colour is drawn only where there is a terminal to draw it
on. `spinloop work ls` is the same list.

## Aborting an item

```sh
spinloop work abort --items ./work.yaml fix-parser
```

Stops a running item: the marker beside the items file the run takes up on its
pass, the item's agent stopped the way a clean interrupt stops it — the polite
signal, the grace, then the hard end — and its record removed from the state.
The item is back in the backlog, and the run's next pass admits it again.

Only a running item can be aborted: an id the file does not carry is refused,
naming it, and an item the state records `backlog`, `done` or `failed` is
refused too, naming the item and its state. Where no orchestrator is running
the file there is nothing running, and the command says so — a stale
`running` record is for the next start to record `failed`, not for abort to
clear. With an orchestrator running, the command leaves its marker and waits
for the run to take it up, reporting the item's state when it does; where the
wait runs out first, or the run dies mid-wait, it says so, naming the item and
the marker that stands beside the file.

## Removing an item

```sh
spinloop work remove --items ./work.yaml docs-refresh
```

Takes an item out of the work list: the item out of the items file, its record
out of the state beside it, and its kept output from the logs beside it — the
file a valid items file after the removal.

A running item cannot be removed: the refusal names it and the abort that goes
first. An id the file does not carry is refused, naming it. `backlog`, `done`
and `failed` items are removed alike, and a running orchestrator picks the
change up on its next pass — the item out of its backlog, and the record
dropped with it, since a record of an item the file no longer carries would
stand in the state and refuse the id's re-add.

## What it does not do

- It takes no lock and talks to no network: the commands work the file and the
  state beside it, and a running orchestrator is reached through the file alone.
- It never runs an agent, and never starts or stops one: an abort asks the run
  to stop its agent, and where there is no run there is nothing to stop.
- It does not re-run an ended item. A `done` or `failed` record stands against
  the id's re-add, the way the orchestrator's does.
- An abort is a stop, not a cancel of the work: the item goes back to the
  backlog and is worked again on a later pass. To keep it out, remove it.

## Flags

| Flag | Meaning |
| ---- | ------- |
| `--items <path>` | The work items file (default `./work.yaml`) — `add`, `list`, `abort`, `remove` |
| `--id <id>` | The item's id — `add` |
| `--instructions <text>` | The instructions the item's agent is given — `add` |
| `--dir <path>` | The directory the agent works in — `add` |
| `--tag <key=value>` | A tag the item carries; repeatable — `add` |
| `--priority <n>` | The item's priority, higher first — `add` |

## See also

- [Working a backlog against the fleet](../work-items.md) — the feature this
  command family works
- [`spinloop orchestrator`](orchestrator.md) — the run the commands hand off
  to, and the [work list API](orchestrator.md#the-work-list-api) the same
  hand-off over HTTP
