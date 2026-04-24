package backend

import (
	"context"
	"os"
	"os/exec"

	"github.com/yearsyan/agentd/internal/config"
)

type ClaudeCode struct{}

func (c *ClaudeCode) Execute(ctx context.Context, prompt string, cfg config.ModelConfig) error {
	cmd := exec.CommandContext(ctx, "claude", "-p", prompt, "--dangerously-skip-permissions")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	if !cfg.Official {
		cmd.Env = append(cmd.Env,
			"ANTHROPIC_BASE_URL="+cfg.BaseURL,
			"ANTHROPIC_AUTH_TOKEN="+cfg.AuthToken,
			"ANTHROPIC_MODEL="+cfg.Model,
		)
		if cfg.SmallFastModel != "" {
			cmd.Env = append(cmd.Env, "ANTHROPIC_SMALL_FAST_MODEL="+cfg.SmallFastModel)
		}
	}
	return cmd.Run()
}
