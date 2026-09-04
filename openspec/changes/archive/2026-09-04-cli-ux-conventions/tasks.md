## 1. The spec

- [x] 1.1 Review the drafted `cli-ux` spec against `cmd/spinloop` command by command, and note any command that already contradicts it — the finding goes in the change, the fix does not
- [x] 1.2 Run `concord check` and `concord overlap` so no requirement here claims something `remote-metrics-bar-format` or `fleet-client` already owns

## 2. The consolidations the spec makes obvious

- [x] 2.1 Move the brand colours and the spinner frames out of `dashboard_render.go` and `dashboard_model.go` into one file of their own under `cmd/spinloop`
- [x] 2.2 Replace `deploySpinnerFrames` in `fleet.go` with that one definition, and check `fleet deploy`'s spinner still draws the same frames in the same order
- [x] 2.3 Correct the one American spelling in `fleet.go`'s `fleet deploy` long description

## 3. Docs and checks

- [x] 3.1 Add `cli-ux` to `AGENTS.md`'s spec pointers, and note in `docs/internals.md` where the palette and the spinner now live
- [x] 3.2 Run `gofmt`, `go vet ./...` and `go test ./... -cover`, keeping total coverage at or above 80%
- [x] 3.3 Run `openspec validate cli-ux-conventions --strict`
