## 1. Daemon history

- [x] 1.1 Add the history shape to `internal/metrics`: per-sample time plus CPU, RAM, and per-GPU percentage readings, and a `History` field on `Stats` omitted when empty
- [x] 1.2 Add the daemon's ring buffer (10-minute window at the sampler cadence) with append, snapshot, and clear operations
- [x] 1.3 Take a system reading on each sampler tick while an engine runs (independent of a scrape target), record a sample on success, record nothing on failure
- [x] 1.4 Clear the buffer on engine start alongside the existing counter/reset hooks; leave it untouched on stop
- [x] 1.5 Include the buffer's contents in `Daemon.Metrics` so `/v1/metrics` reports the history
- [x] 1.6 Unit tests: buffer wrap and ordering, sample on success / none on failure, clear on start, persistence across stop, exposure on the metrics reply

## 2. API contract

- [x] 2.1 Add the history field to the metrics response in `docs/openapi.yaml`
- [x] 2.2 Confirm `openapi_test.go` passes with the new field

## 3. CLI rendering

- [x] 3.1 Add the sparkline renderer: eight block glyphs, max-pool downsampling to the draw width, latest point coloured on the 80/90 thresholds, trailing percentage
- [x] 3.2 Rename the existing filled-bar renderer to the gauge role and route `--format` on both `remote metrics` and `fleet metrics` across `bar`, `gauge`, `table`, and `json`
- [x] 3.3 Draw the bar format from the daemon's history for the same series and labels the gauge draws, with the gauge-style fallback where a daemon reports no history
- [x] 3.4 Keep the bar format drawing the retained history for a stopped engine (ending at the stop), with the existing last-active and header behaviour
- [x] 3.5 Tests: renderer glyphs, downsampling keeps peaks, colour thresholds on the last point, format validation errors, no-history fallback, stopped-engine output

## 4. Dashboard

- [x] 4.1 Add the board-wide format state and the `g` toggle to the dashboard model, opening in bar, with the key help naming it
- [x] 4.2 Draw each tile's resource series in the board's current format from the node's history, reusing the fallback for nodes whose daemon reports none
- [x] 4.3 Tests: toggle switches every tile and back, tiles keep their geometry in both formats, nodes without history fall back

## 5. Remote relay

- [x] 5.1 Add the history types to `remote/lambda/shared/daemon.ts` and `shared/stats.ts`
- [x] 5.2 Copy the history field through in the stats Lambda, unaltered
- [x] 5.3 TypeScript tests covering the relay of the field and its absence

## 6. Documentation

- [x] 6.1 Update the command reference under `docs/` for the `bar`/`gauge` formats and the dashboard's `g` key

## 7. Verification

- [x] 7.1 `gofmt`, `go vet ./...`, and `go test ./... -cover` with total coverage at or above 80%
- [x] 7.2 The `remote/` pnpm suite passes
