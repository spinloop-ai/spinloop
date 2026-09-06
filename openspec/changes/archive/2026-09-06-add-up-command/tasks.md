## 1. Implementation

- [x] 1.1 Add `upCmd()` in a new `cmd/spinloop/up.go` — `Use: "up"`, a
      lowercase imperative `Short`, a `Long` stating the dispatch rule,
      positional arguments only — and register it beside `serveCmd()` in
      `cmd/spinloop/commands.go`
- [x] 1.2 Fleet branch: when `./fleet.yaml` exists in the current directory,
      run the fleet start path over the named nodes, or over every node when
      none are given; reuse `fleet.Resolve` and the existing start call and
      result rendering so the output is `fleet start`'s
- [x] 1.3 Serve branch: otherwise run `serve`'s own body with the positional
      (or none), so resolution, output, and errors are serve's
- [x] 1.4 Completion slot: the fleet's node names when `./fleet.yaml` is
      present, otherwise the Spinloop slot (alias names plus paths), silent on
      any failure

## 2. Tests

- [x] 2.1 In a fleet directory: bare `up` starts every node; `up <node>` starts
      the named one; an unknown node name fails as `fleet start` does
- [x] 2.2 Outside a fleet directory: `up` serves `./Spinloop`; `up <path>`
      serves the path; a `SPINLOOP_ALIAS` value and a registered alias both
      resolve as they do for `serve`; nothing resolvable fails with serve's
      "no Spinloop found" error
- [x] 2.3 A `fleet.yaml` and a Spinloop in the same directory: the fleet wins
- [x] 2.4 The completion slot offers node names in a fleet directory and the
      Spinloop slot elsewhere, and stays silent on an unreadable fleet file

## 3. Documentation

- [x] 3.1 Add `docs/commands/up.md`: the dispatch rule, the two forms, and a
      pointer to `serve` and `fleet start` for the full options
- [x] 3.2 Add `spinloop up` to `docs/README.md`'s command table

## 4. Verification

- [x] 4.1 `go test ./... -cover` passes with total coverage >= 80%
- [x] 4.2 `go vet ./...` and `gofmt -w ./...` clean
- [x] 4.3 By hand: in a Spinloop directory, `up` prints and runs the serve
      command; `up --help` states the rule; where a fleet is available, `up`
      in its directory starts the nodes and `fleet status` shows them
      (fleet branch verified against stub daemons in 2.1 — no live fleet
      available by hand)
