//go:build windows

package config

func defaultSocketPath() string {
	return "127.0.0.1:19800"
}
