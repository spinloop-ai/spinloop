## Context

See proposal.md. Two questions this design settles: where in the fleet file
the gateway lives, and what the new command does with it.

## Goals / Non-Goals

**Goals:**

- The gateway is a property of the fleet: the fleet file names it, and the
  command at the fleet's level uses it.
- `spinloop fleet harness` reuses the launch path's routing wholesale — no
  second selector, no second wake path.

**Non-Goals:**

- Removing `FLEET` from Spinloops: the follow-up breaking change.
- `spinloop gateway` reading its own section for its token: a natural
  follow-up, not part of this change.

## Decisions

### A section, not a node kind

A node kind would put the gateway inside the fleet's machinery: every fleet
operation — the status fan-out, start/stop, selection, ranking, wake — would
special-case an entry it cannot act on. A gateway has no control API, nothing
in the fleet starts or stops it, and it must never be a selection candidate,
since it serves whatever its nodes serve. A top-level section costs the
machinery nothing: the fleet's commands ignore it, and only the paths that
need a gateway read it. Singular also reads correctly — a fleet has one
canonical front door, and extra gateways are operational choices, reached by
naming their URL.

### The command's file wins, then the Spinloop's FLEET, then the default

An explicit `-f` wins over a Spinloop's `FLEET`, generalising the existing
`--fleet`-overrides-instruction rule; with no `-f`, the Spinloop's `FLEET`
(file or endpoint) is used as today; with neither, `./fleet.yaml` is read. One
rule covers all four sources, and every existing launch keeps behaving exactly
as it does.

### The section routes the way an endpoint routes

A file naming a gateway produces the same answer an endpoint `FLEET` does —
the address as the base URL with the OpenAI-compatible prefix, the token
through the client's key chain, no node contacted — differing only in where
the address and the variable come from: the section rather than the
instruction. The token's default variable is `OPENAI_API_KEY`, the one an
endpoint already resolves under.

### The command is a thin front on the launch path

`spinloop fleet harness` resolves its two inputs (Spinloop, fleet file) and
then runs the launch path's routing — the gateway branch, selection and wake,
the `BASEURL` and environment rules, the stderr announcement — rather than
re-implementing any of it. Its one behaviour the launch path lacks: with no
Spinloop at all it fails, because routing needs a model to route.
