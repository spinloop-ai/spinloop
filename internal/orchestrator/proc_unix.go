//go:build !windows

package orchestrator

import (
	"os"
	"os/exec"
	"syscall"
)

// setProcAttr puts the agent in its own process group, so stopping the
// orchestrator reaches the agent and anything it spawned rather than the
// orchestrator's own group.
func setProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// terminateGroup stops the agent's process group with the polite signal
// first, the group's lead process as the fallback.
func terminateGroup(p *os.Process) {
	if err := syscall.Kill(-p.Pid, syscall.SIGTERM); err != nil {
		p.Signal(syscall.SIGTERM)
	}
}

// killGroup ends the agent's process group hard, after the grace a Stop has
// had.
func killGroup(p *os.Process) {
	if err := syscall.Kill(-p.Pid, syscall.SIGKILL); err != nil {
		p.Kill()
	}
}

// alive reports whether a process with this pid is running: the liveness
// check a stale lock is judged by.
func alive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = p.Signal(syscall.Signal(0))
	if err == nil {
		return true
	}
	// EPERM means the process exists and belongs to someone else.
	return err == syscall.EPERM
}
