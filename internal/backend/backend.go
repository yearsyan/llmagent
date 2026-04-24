package backend

import (
	"context"
	"fmt"

	"github.com/yearsyan/agentd/internal/config"
)

type Backend interface {
	Execute(ctx context.Context, prompt string, cfg config.ModelConfig) error
}

func For(cfg config.ModelConfig) (Backend, error) {
	switch cfg.Backend {
	case config.BackendClaudeCode:
		return &ClaudeCode{}, nil
	case config.BackendOpenCode:
		return &OpenCode{}, nil
	case config.BackendCodex:
		return &Codex{}, nil
	default:
		return nil, fmt.Errorf("unknown backend: %s", cfg.Backend)
	}
}
