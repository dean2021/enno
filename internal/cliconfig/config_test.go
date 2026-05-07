package cliconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseRequiresModel(t *testing.T) {
	isolateHome(t)

	_, err := Parse([]string{"run", "hello"})
	if err == nil || !strings.Contains(err.Error(), "config.yaml") {
		t.Fatalf("expected missing model error, got %v", err)
	}
}

func TestParseCreatesDefaultConfigFileWhenMissing(t *testing.T) {
	home := isolateHome(t)

	_, err := Parse([]string{"run", "hello"})
	if err == nil || !strings.Contains(err.Error(), "config.yaml") {
		t.Fatalf("expected missing model error, got %v", err)
	}

	path := filepath.Join(home, ".enno", "config.yaml")
	content, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("expected default config file to be created: %v", readErr)
	}
	if !strings.Contains(string(content), "Enno CLI configuration") {
		t.Fatalf("expected template config content, got %q", string(content))
	}
}

func TestParseRequiresOpenAIBaseURL(t *testing.T) {
	isolateHome(t)
	configPath := writeConfig(t, `
provider: openai
model: test-model
`)

	_, err := Parse([]string{"run", "--config", configPath, "hello"})
	if err == nil || !strings.Contains(err.Error(), "base_url") {
		t.Fatalf("expected missing base URL error, got %v", err)
	}
}

func TestParseAnthropicDoesNotRequireBaseURL(t *testing.T) {
	isolateHome(t)
	configPath := writeConfig(t, `
provider: anthropic
model: claude-test
api_key: test-key
`)

	cfg, err := Parse([]string{"run", "--config", configPath, "hello"})
	if err != nil {
		t.Fatalf("expected anthropic config without base URL, got %v", err)
	}
	if cfg.Mode != "run" || cfg.Query != "hello" {
		t.Fatalf("unexpected parsed config: mode=%q query=%q", cfg.Mode, cfg.Query)
	}
}

func TestParseOpenAIWithRequiredConfig(t *testing.T) {
	isolateHome(t)
	configPath := writeConfig(t, `
provider: openai
model: test-model
base_url: https://example.com/v1
`)

	cfg, err := Parse([]string{"run", "--config", configPath, "hello"})
	if err != nil {
		t.Fatalf("expected openai config, got %v", err)
	}
	if cfg.Mode != "run" || cfg.Query != "hello" {
		t.Fatalf("unexpected parsed config: mode=%q query=%q", cfg.Mode, cfg.Query)
	}
}

func TestParseExplicitConfigMissingFile(t *testing.T) {
	isolateHome(t)

	_, err := Parse([]string{"run", "--config", filepath.Join(t.TempDir(), "missing.yaml"), "hello"})
	if err == nil || !strings.Contains(err.Error(), "read config file") {
		t.Fatalf("expected missing explicit config error, got %v", err)
	}
}

func TestParseReadsOpenAIConfigFile(t *testing.T) {
	isolateHome(t)
	configPath := writeConfig(t, `
provider: openai
model: yaml-model
api_key: yaml-key
base_url: https://yaml.example/v1
max_tokens: 2048
`)

	cfg, err := Parse([]string{"run", "--config", configPath, "hello"})
	if err != nil {
		t.Fatalf("expected config file to parse, got %v", err)
	}
	if cfg.Mode != "run" || cfg.Query != "hello" {
		t.Fatalf("unexpected parsed config: mode=%q query=%q", cfg.Mode, cfg.Query)
	}
}

func TestParseIgnoresEnvironmentVariables(t *testing.T) {
	isolateHome(t)
	configPath := writeConfig(t, `
provider: anthropic
model: yaml-model
api_key: yaml-key
`)
	t.Setenv("ENNO_PROVIDER", "openai")
	t.Setenv("ENNO_MODEL", "env-model")
	t.Setenv("ENNO_BASE_URL", "")
	t.Setenv("ANTHROPIC_API_KEY", "")

	cfg, err := Parse([]string{"run", "--config", configPath, "hello"})
	if err != nil {
		t.Fatalf("expected env vars to be ignored in favor of config file, got %v", err)
	}
	if cfg.Mode != "run" || cfg.Query != "hello" {
		t.Fatalf("unexpected parsed config: mode=%q query=%q", cfg.Mode, cfg.Query)
	}
}

func TestParseNoLongerAcceptsProviderConfigFlags(t *testing.T) {
	isolateHome(t)
	configPath := writeConfig(t, `
provider: openai
model: yaml-model
base_url: https://yaml.example/v1
`)

	_, err := Parse([]string{"run", "--config", configPath, "--model", "flag-model", "hello"})
	if err == nil || !strings.Contains(err.Error(), "flag provided but not defined") {
		t.Fatalf("expected provider config flag to be rejected, got %v", err)
	}
}

func TestParseAnthropicConfigFileDoesNotRequireBaseURL(t *testing.T) {
	isolateHome(t)
	configPath := writeConfig(t, `
provider: anthropic
model: yaml-claude
api_key: yaml-key
`)

	cfg, err := Parse([]string{"run", "--config", configPath, "hello"})
	if err != nil {
		t.Fatalf("expected anthropic config file without base URL, got %v", err)
	}
	if cfg.Mode != "run" || cfg.Query != "hello" {
		t.Fatalf("unexpected parsed config: mode=%q query=%q", cfg.Mode, cfg.Query)
	}
}

func TestParseConfigCanDisableTools(t *testing.T) {
	isolateHome(t)
	configPath := writeConfig(t, `
provider: anthropic
model: yaml-claude
shell: false
filesystem: false
`)

	cfg, err := Parse([]string{"run", "--config", configPath, "hello"})
	if err != nil {
		t.Fatalf("expected config file, got %v", err)
	}
	if got := len(cfg.AgentConfig.Tools); got != 1 {
		t.Fatalf("expected only todo tool, got %d tools", got)
	}
}

func isolateHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("ENNO_PROVIDER", "")
	t.Setenv("ENNO_MODEL", "")
	t.Setenv("ENNO_API_KEY", "")
	t.Setenv("ENNO_BASE_URL", "")
	t.Setenv("ENNO_MAX_TOKENS", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	return home
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
