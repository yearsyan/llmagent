package skill

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed template/SKILL.md
var templateFS embed.FS

func Install() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("home dir: %w", err)
	}

	targets := []string{
		filepath.Join(home, ".claude", "skills"),
		filepath.Join(home, ".codex", "skills"),
	}

	var found []string
	for _, skillsDir := range targets {
		if info, err := os.Stat(skillsDir); err != nil || !info.IsDir() {
			continue
		}
		found = append(found, skillsDir)
	}

	if len(found) == 0 {
		return nil, fmt.Errorf("no skills directory found (~/.claude/skills or ~/.codex/skills)")
	}

	content, err := templateFS.ReadFile("template/SKILL.md")
	if err != nil {
		return nil, fmt.Errorf("read template: %w", err)
	}

	var written []string
	for _, skillsDir := range found {
		dir := filepath.Join(skillsDir, "llmagent")
		if err := os.MkdirAll(dir, 0755); err != nil {
			return written, fmt.Errorf("create dir %s: %w", dir, err)
		}
		path := filepath.Join(dir, "SKILL.md")
		if err := os.WriteFile(path, content, 0644); err != nil {
			return written, fmt.Errorf("write %s: %w", path, err)
		}
		written = append(written, path)
	}

	return written, nil
}
