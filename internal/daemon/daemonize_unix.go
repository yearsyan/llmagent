//go:build !windows

package daemon

import (
	"os/exec"
	"syscall"
)

func SetDaemonAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // Create new session, detach from controlling terminal
	}
	// Close stdin so daemon doesn't tie to caller's terminal
	cmd.Stdin = nil
}
