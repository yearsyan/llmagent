//go:build !windows

package daemon

import (
	"net"
	"os"
)

func listen(addr string) (net.Listener, error) {
	if err := os.Remove(addr); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return net.Listen("unix", addr)
}

func dial(addr string) (net.Conn, error) {
	return net.Dial("unix", addr)
}

func Cleanup(addr string) {
	os.Remove(addr)
	os.Remove(addr + ".pid")
}
