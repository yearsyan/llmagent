package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type BackendType string

const (
	BackendClaudeCode BackendType = "claude-code"
	BackendOpenCode   BackendType = "opencode"
	BackendCodex      BackendType = "codex"
)

type ModelConfig struct {
	Backend        BackendType `yaml:"backend"`
	Official       bool        `yaml:"official"`
	BaseURL        string      `yaml:"base_url"`
	AuthToken      string      `yaml:"auth_token"`
	Model          string      `yaml:"model"`
	SmallFastModel string      `yaml:"small_fast_model"`
	Description    string      `yaml:"description"`
}

type DaemonConfig struct {
	Socket string `yaml:"socket"`
}

type SummaryConfig struct {
	BaseURL string `yaml:"base_url"`
	APIKey  string `yaml:"api_key"`
	Model   string `yaml:"model"`
}

type Config struct {
	Models        map[string]ModelConfig `yaml:"models"`
	Daemon        DaemonConfig           `yaml:"daemon"`
	Summary       SummaryConfig          `yaml:"summary"`
	SummaryPrompt string                 `yaml:"summary_prompt"`
	LoadedPath    string                 `yaml:"-"`
}

func defaultConfigPath() string {
	if p := os.Getenv("LLMAGENT_CONFIG"); p != "" {
		return p
	}
	// Dev mode: only when LLM_AGENT_DEV=1
	if os.Getenv("LLM_AGENT_DEV") == "1" {
		if _, err := os.Stat("config.dev.yaml"); err == nil {
			return "config.dev.yaml"
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".llmagent", "config.yaml")
}

func Load() (*Config, error) {
	path := defaultConfigPath()
	if path == "" {
		return nil, fmt.Errorf("cannot determine config path: set LLMAGENT_CONFIG")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config file not found at %s: create one with model→backend mappings", path)
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	cfg.LoadedPath = path
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if cfg.Models == nil {
		cfg.Models = make(map[string]ModelConfig)
	}
	if cfg.Daemon.Socket == "" {
		cfg.Daemon.Socket = defaultSocketPath()
	} else {
		cfg.Daemon.Socket = expandPath(cfg.Daemon.Socket)
	}

	return &cfg, nil
}

func (c *Config) GetModelConfig(modelID string) (ModelConfig, error) {
	mc, ok := c.Models[modelID]
	if !ok {
		return ModelConfig{}, fmt.Errorf("model %q not found in config", modelID)
	}
	if mc.Backend == "" {
		return ModelConfig{}, fmt.Errorf("model %q has no backend configured", modelID)
	}
	if mc.Backend == BackendClaudeCode {
		if mc.Official {
			return mc, nil
		}

		var missing []string
		if mc.BaseURL == "" {
			missing = append(missing, "base_url")
		}
		if mc.AuthToken == "" {
			missing = append(missing, "auth_token")
		}
		if mc.Model == "" {
			missing = append(missing, "model")
		}
		if len(missing) > 0 {
			return ModelConfig{}, fmt.Errorf("claude-code model %q must set official: true for official Claude Code, or configure %s", modelID, strings.Join(missing, ", "))
		}
	}
	return mc, nil
}

const DefaultSummaryPrompt = `You are a summarization assistant for LLM agent outputs. Summarize the following agent session output concisely. Focus on:
1. What was accomplished
2. Key decisions or findings
3. Any errors or issues
4. Next steps if mentioned

Keep the summary clear, structured, and under 500 words.`

// GetSummaryPrompt returns the configured summary prompt or the built-in default.
func (c *Config) GetSummaryPrompt() string {
	if c.SummaryPrompt != "" {
		return c.SummaryPrompt
	}
	return DefaultSummaryPrompt
}

// HasSummaryConfig returns true if the summary API is configured.
func (c *Config) HasSummaryConfig() bool {
	return c.Summary.BaseURL != "" && c.Summary.APIKey != ""
}

func expandPath(p string) string {
	if len(p) > 1 && p[0] == '~' && p[1] == '/' {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}
