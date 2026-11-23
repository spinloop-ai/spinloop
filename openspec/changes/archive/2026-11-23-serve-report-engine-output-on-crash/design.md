# Design

## Context

See proposal.md — Why. The view path (`runServeView`) runs the engine through
`startSupervisedForeground` with the supervisor's `LogPath` set to the state-dir
`engine.log`; every byte the engine writes lands there. When the engine dies at
startup the view closes on `serveViewEngineExitedMsg`, `run.wait()` returns the
exit error, and `main` prints `Error: exit status N` — the captured output never
reaches the console. The piped run forwards stdio live and needs nothing.

The pieces a report needs already exist: the supervisor classifies every exit
(`StateStopped` for a stop on request or a clean exit, `StateCrashed` otherwise),
the run's `wait()` already maps "crashed" to a non-nil error and "stopped or
clean" to nil, and `daemon.ReadLog` is the existing bounded, whole-line log read
the view and the control API both use.

## Goals / Non-Goals

**Goals:**
- A crashed engine's own words are on the console when serve exits, with a
  pointer to the full log.
- The report is deterministic and testable without a terminal, like the rest of
  the serve suite.

**Non-Goals:**
- Changing which paths capture the log (only the view run does).
- Touching the daemon's crash record, `spinloop daemon logs`, or fleet/cloud
  surfaces.
- Keeping the view open until the operator acknowledges a crash — the spec says
  the engine's exit closes the view, and it still does.

## Decisions

- **Hook where the run's result is already classified.** In `runServeView`,
  capture `run.wait()`'s error; when it is non-nil, write the log tail to
  stderr before returning it. `wait()`'s existing rule (`StateStopped` → nil)
  is exactly "stopped on request or exited clean", so no new classification
  logic is needed, and the stop/clean paths print nothing extra by
  construction. Alternative considered: classify via `sup.Status()` again at the
  call site — rejected, it would restate a rule `wait()` already owns.
- **Read the tail through `daemon.ReadLog(logPath, daemon.TailLog, budget)`.**
  Same bounded, whole-line semantics the view's backlog uses; no new file
  reading. Budget: a console-scale 40 lines (`40 * bytesPerLineGuess`), well
  below the view's 200-line backlog budget — this is a last words dump, not a
  pager; the header names the file for the whole record.
- **A small standalone helper** (`reportEngineLogTail(w io.Writer, logPath string)`)
  owns the writing: header naming the log path, then the tail verbatim, then a
  plain note when the log is empty or unreadable. A plain writer keeps it unit
  testable without a terminal or a view; `runServeView` passes `os.Stderr`.
  Alternative considered: fold the tail into the returned error so `main`
  prints one message — rejected: the exit status stays the run's result, and
  stdout/stderr narration (which the view path sends to stderr anyway) stays
  the place where serve talks to the operator.
- **Plain text, no colour.** The tail is the engine's bytes, verbatim; the
  header uses the tool's usual plain narration. The view has already left the
  alternate screen, so this is ordinary stderr output.

## Risks / Trade-offs

- [A very noisy engine can fill 40 lines with unrelated spam before its error]
  → The header names the full log path for the complete record; 40 lines is
  enough for any engine's startup-failure output in practice.
- [The engine may still be flushing when the view closes] → No mitigation
  needed: `wait()` returns only after the supervisor's `cmd.Wait()` and the
  PTY drain have finished writing the log, so the file is complete and closed.
- [stderr may be redirected while stdout was the terminal] → Acceptable: the
  report is for the operator who ran serve; a redirected stderr is where they
  sent it.
