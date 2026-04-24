//go:build windows

package daemon

import (
	"os"
	"os/signal"
)

func TerminateProcess(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Signal(os.Kill)
}

func NotifyShutdown(ch chan<- os.Signal) {
	signal.Notify(ch, os.Interrupt)
}
