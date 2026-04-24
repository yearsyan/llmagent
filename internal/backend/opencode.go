package backend

import (
	"context"
	"os"
	"os/exec"

	"github.com/yearsyan/agentd/internal/config"
)

type OpenCode struct{}

func (o *OpenCode) Execute(ctx context.Context, prompt string, cfg config.ModelConfig) error {
	args := []string{"run", prompt}
	if cfg.Model != "" {
		args = append(args, "--model", cfg.Model)
	}
	cmd := exec.CommandContext(ctx, "opencode", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
