//go:build windows

package orchestrator

import (
	"os"
	"os/exec"
)

// setProcAttr is a no-op on Windows, which has no process groups to join.
func setProcAttr(cmd *exec.Cmd) {}

// terminateGroup has no polite cross-process signal on Windows; the agent
// is killed outright.
func terminateGroup(p *os.Process) {
	p.Kill()
}

// killGroup has no process groups on Windows; the agent is killed outright.
func killGroup(p *os.Process) {
	p.Kill()
}

// alive cannot be told on Windows without the process handle, so a lock is
// never refused on liveness grounds: a stale lock is taken over.
func alive(pid int) bool { return pid == os.Getpid() }
