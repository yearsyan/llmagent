//go:build windows

package daemon

import "os/exec"

func SetDaemonAttr(cmd *exec.Cmd) {
	cmd.Stdin = nil
}
