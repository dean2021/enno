package loadskill

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// frontMatter is the optional YAML block at the top of SKILL.md.
type frontMatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// parseSkillFile reads path (a SKILL.md file) and returns display name, description, and body.
// defaultName is used when front matter omits name (typically the parent directory name).
func parseSkillFile(path, defaultName string) (name, description, body string, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", "", err
	}
	text := string(data)

	meta, body, ok := splitFrontmatter(text)
	if !ok {
		return strings.TrimSpace(defaultName), "", strings.TrimSpace(text), nil
	}
	var fm frontMatter
	if err := yaml.Unmarshal([]byte(meta), &fm); err != nil {
		return "", "", "", fmt.Errorf("skill frontmatter %s: %w", path, err)
	}
	name = strings.TrimSpace(fm.Name)
	if name == "" {
		name = strings.TrimSpace(defaultName)
	}
	description = strings.TrimSpace(fm.Description)
	body = strings.TrimSpace(body)
	return name, description, body, nil
}

// splitFrontmatter expects leading "---\n" ... "---\n" body; returns yaml fragment and body.
func splitFrontmatter(text string) (yamlBlock string, body string, ok bool) {
	text = strings.TrimPrefix(text, "\ufeff") // BOM
	if !strings.HasPrefix(text, "---") {
		return "", "", false
	}
	rest := strings.TrimPrefix(text, "---")
	rest = strings.TrimPrefix(rest, "\n")
	rest = strings.TrimPrefix(rest, "\r\n")

	idx := strings.Index(rest, "---")
	if idx < 0 {
		return "", "", false
	}
	meta := strings.TrimSuffix(rest[:idx], "\r")
	meta = strings.TrimRight(meta, "\n")

	after := rest[idx+3:]
	after = strings.TrimPrefix(after, "\n")
	after = strings.TrimPrefix(after, "\r\n")

	return strings.TrimSpace(meta), after, true
}
