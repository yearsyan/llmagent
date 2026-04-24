package backend

import (
	"context"
	"os"
	"os/exec"

	"github.com/yearsyan/agentd/internal/config"
)

type Codex struct{}

func (c *Codex) Execute(ctx context.Context, prompt string, cfg config.ModelConfig) error {
	args := []string{"exec", prompt, "--skip-git-repo-check"}
	if cfg.Model != "" {
		args = append(args, "--model", cfg.Model)
	}
	cmd := exec.CommandContext(ctx, "codex", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
