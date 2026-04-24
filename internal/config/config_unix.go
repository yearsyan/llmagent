//go:build !windows

package config

import (
	"os"
	"path/filepath"
)

func defaultSocketPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/llmagent.sock"
	}
	return filepath.Join(home, ".llmagent", "llmagent.sock")
}
