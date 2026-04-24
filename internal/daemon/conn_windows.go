//go:build windows

package daemon

import (
	"net"
	"os"
)

func listen(addr string) (net.Listener, error) {
	return net.Listen("tcp", addr)
}

func dial(addr string) (net.Conn, error) {
	return net.Dial("tcp", addr)
}

func Cleanup(addr string) {
	os.Remove(addr + ".pid")
}
