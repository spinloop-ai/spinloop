# Tasks

## 1. The sampler's reading serves the metrics endpoint

- [x] 1.1 Record each system reading on the daemon as the current reading — the figures, the collection's failures, and whether a reading has landed — from the sampler's per-tick collection, drop it on a start after a stop, and copy it (not a fresh collection) onto the metrics response, so an unsampled figure stays absent and the handler runs no host commands, verified by `TestSystemSampleOnce` (a reading is recorded and applied), `TestSystemSampleOnceRecordsNothingOnFailure` (a failed reading is still recorded, with its failures), `TestSystemHistoryClearsOnStartAndSurvivesAStop` (the reading is dropped on a start after a stop), and `TestMetricsRunsNoHostCommands` (the handler takes no collection)
- [x] 1.2 Report a reading's collection failures among the metrics response's errors, naming the source, so a broken host source is reported from the reading now that the handler no longer collects, verified by `TestSampledCollectionErrorsReachMetrics`
- [x] 1.3 Bound the short catch-up interval with a known scrape target — it applies only while a target is known and no counters have come back, so an engine whose runner exposes no metrics endpoint settles at the tick instead of running the host commands every second — verified by `TestNoScrapeTargetLeavesTheCatchUpInterval`

## 2. The dashboard's cadence

- [x] 2.1 Move the dashboard's local refresh interval from 2 seconds to 5 — one call per machine per tick, the figures it draws are sampled every 15 seconds anyway — verified by the dashboard refresh tests passing against the new constant
