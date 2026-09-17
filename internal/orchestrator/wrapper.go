package orchestrator

import (
	"errors"
	"fmt"
	"strconv"
)

// wrapperSentinelExit is the lifecycle wrapper's own signal that the
// startup script failed before the harness ever ran — read only where the
// wrapper actually ran (HarnessConfig.hasLifecycle), so it is never
// confused with a harness's own exit code on a run with no harness.yaml,
// or one naming neither script.
const wrapperSentinelExit = 97

// wrapperScript is the POSIX shell run in place of the harness whenever
// HarnessConfig names a startup or shutdown script: it runs STARTUP first,
// the harness itself (given as its own arguments) only where that
// succeeds, then SHUTDOWN once the harness has ended — whatever ended it.
// term forwards an incoming stop signal to the harness child rather than
// letting the wrapper's own default disposition take it down before
// shutdown gets to run; see design.md's "The wrapper" decision. Both
// backends run this same script — the bare backend as a local process, the
// docker backend as the container's command — assuming a POSIX shell is on
// the PATH either way.
var wrapperScript = `set -u
term() { [ -n "${child:-}" ] && kill -TERM "$child" 2>/dev/null; }
trap term TERM INT

sh -c "$STARTUP"; startup_rc=$?
if [ "$startup_rc" -ne 0 ]; then
	sh -c "$SHUTDOWN"
	exit ` + strconv.Itoa(wrapperSentinelExit) + `
fi

"$@" & child=$!
wait "$child"; harness_rc=$?
# A trap firing while wait was blocked can make it return early with
# 128+signal rather than the child's real status (a POSIX/bash quirk, not
# the child's own outcome); wait again for the same pid, now already
# reaped, to read the real one.
if [ "$harness_rc" -ge 128 ]; then
	wait "$child" 2>/dev/null; harness_rc=$?
fi

sh -c "$SHUTDOWN"
exit "$harness_rc"
`

// wrapCommand returns the bin, args and extra environment a launch should
// actually run: the harness directly where hc names neither script, or the
// lifecycle wrapper around it, carrying STARTUP/SHUTDOWN in its
// environment, otherwise.
func wrapCommand(bin string, args []string, hc HarnessConfig) (runBin string, runArgs []string, extraEnv []string) {
	if !hc.hasLifecycle() {
		return bin, args, nil
	}
	runArgs = append([]string{"-c", wrapperScript, "spinloop-wrapper", bin}, args...)
	return "/bin/sh", runArgs, []string{"STARTUP=" + hc.Startup, "SHUTDOWN=" + hc.Shutdown}
}

// exitCoder is what both *exec.ExitError and dockerExitError implement:
// the numeric exit code an ended process left, however the backend that
// ran it reports one.
type exitCoder interface{ ExitCode() int }

// sentinelChild wraps a Child running under the lifecycle wrapper: its
// Wait turns the wrapper's own sentinel exit into a startup failure,
// naming the script, rather than reporting it as if it were the harness's
// own outcome. Every other outcome — the harness's own exit, a clean
// finish — passes through unchanged.
type sentinelChild struct{ Child }

func (c sentinelChild) Wait() error {
	err := c.Child.Wait()
	var ec exitCoder
	if errors.As(err, &ec) && ec.ExitCode() == wrapperSentinelExit {
		return fmt.Errorf("the startup script failed before the harness ran")
	}
	return err
}

// wrapChild wraps child in a sentinelChild where hc names a startup or
// shutdown script, so its exit is read the wrapper's way; where hc names
// neither, child is returned unchanged — a run with no harness.yaml, or
// one naming no script, works exactly as it always has.
func wrapChild(child Child, hc HarnessConfig) Child {
	if !hc.hasLifecycle() {
		return child
	}
	return sentinelChild{child}
}
