## 1. Flag registration

- [ ] 1.1 Add the `-f` short form to the `--fleet` flag of the six fleet
      subcommands (`status`, `metrics`, `start`, `stop`, `deploy`, `route`) in
      `cmd/spinloop/fleet.go`, changing each registration from `StringVar` to
      `StringVarP` with shorthand `f`.
- [ ] 1.2 Add the `-f` short form to `fleet dashboard`'s `--fleet` flag in
      `cmd/spinloop/fleet_dashboard.go` the same way.
- [ ] 1.3 Add the `-f` short form to `harness`'s `--fleet` flag in
      `cmd/spinloop/commands.go`.
- [ ] 1.4 Leave `fleet logs` (`cmd/spinloop/fleet_logs.go`) on `StringVar`
      for `--fleet` — no shorthand, since `-f` is that command's `--follow` —
      with a comment saying the short form is unavailable there and why.

## 2. Completion

- [ ] 2.1 In `cmd/spinloop/complete.go`, add `"-f"` to `harnessValueFlags` so
      the harness slot logic treats `-f <path>` as consuming the path word.
- [ ] 2.2 In `harnessSlot`'s detached-flag value case, offer the file-path
      directive for `"-f"` alongside `--providers`/`--fleet`.

## 3. Tests

- [ ] 3.1 Test that the `fleet` flag on each of the eight commands carries
      shorthand `f` (walk the command tree's flag sets and assert
      `Lookup("fleet").Shorthand`), and that `fleet logs`'s `fleet` flag has
      no shorthand while its `follow` flag keeps `-f`.
- [ ] 3.2 Test `-f <path>` end to end on a representative subcommand:
      `fleet status -f <path>` (through the existing stub-node fixture) reads
      the named file rather than `./fleet.yaml`.
- [ ] 3.3 Test that `fleet logs -f --fleet <path>` runs in follow mode
      against the named file, and that `fleet logs -f <name>` treats `<name>`
      as a node (unknown-node error for a name not in the file).
- [ ] 3.4 Test that `harness -f <path>` routes through the named fleet file,
      mirroring the existing `--fleet` routing test.
- [ ] 3.5 Test that `harness -f <TAB>` completes the fleet-file value as file
      paths (the default directive), through the existing `complete` test
      seam.

## 4. Docs

- [ ] 4.1 Update the flags table in `docs/commands/fleet.md`: `--fleet`
      becomes `-f, --fleet <path>`, with the table's `-f, --follow` row noting
      that `logs` takes its fleet file long-form only because `-f` is its
      follow flag.
- [ ] 4.2 Update the `--fleet` row in the flags table in
      `docs/commands/harness.md` to show `-f, --fleet`.

## 5. Verification

- [ ] 5.1 `go build ./...`, `go vet ./...`, `gofmt -l .`, and
      `go test ./...` are all clean.
- [ ] 5.2 `go test ./... -cover` keeps total coverage at or above 80%.
