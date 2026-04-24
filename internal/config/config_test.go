package config

import (
	"strings"
	"testing"
)

func TestGetModelConfigAllowsExplicitOfficialClaudeCode(t *testing.T) {
	cfg := &Config{
		Models: map[string]ModelConfig{
			"claude-official": {
				Backend:  BackendClaudeCode,
				Official: true,
			},
		},
	}

	mc, err := cfg.GetModelConfig("claude-official")
	if err != nil {
		t.Fatalf("GetModelConfig returned error: %v", err)
	}
	if !mc.Official {
		t.Fatal("expected official Claude Code config")
	}
}

func TestGetModelConfigRejectsImplicitOfficialClaudeCode(t *testing.T) {
	cfg := &Config{
		Models: map[string]ModelConfig{
			"claude-implicit": {
				Backend: BackendClaudeCode,
			},
		},
	}

	_, err := cfg.GetModelConfig("claude-implicit")
	if err == nil {
		t.Fatal("expected missing claude-code config error")
	}
	if !strings.Contains(err.Error(), "official: true") {
		t.Fatalf("expected error to mention official: true, got %q", err)
	}
}

func TestGetModelConfigAllowsConfiguredClaudeCodeProxy(t *testing.T) {
	cfg := &Config{
		Models: map[string]ModelConfig{
			"deepseek": {
				Backend:   BackendClaudeCode,
				BaseURL:   "https://api.example.com/anthropic",
				AuthToken: "sk-test",
				Model:     "deepseek-reasoner",
			},
		},
	}

	mc, err := cfg.GetModelConfig("deepseek")
	if err != nil {
		t.Fatalf("GetModelConfig returned error: %v", err)
	}
	if mc.Official {
		t.Fatal("did not expect official Claude Code config")
	}
}
