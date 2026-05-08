package cliconfig

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/dean2021/enno/tools/glob"
	"github.com/dean2021/enno/tools/grep"
	"github.com/dean2021/enno/tools/taskgraph"
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
	if !strings.Contains(string(content), "Enno CLI") {
		t.Fatalf("expected template config content, got %q", string(content))
	}
	if !strings.Contains(string(content), "compaction:") || !strings.Contains(string(content), "enabled: true") {
		t.Fatalf("expected default template to enable compaction, got %q", string(content))
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

func TestParseProxyAlias(t *testing.T) {
	isolateHome(t)
	configPath := writeConfig(t, `
provider: anthropic
model: claude-test
api_key: test-key
proxy: socks5://127.0.0.1:7891
`)

	_, err := Parse([]string{"run", "--config", configPath, "hello"})
	if err != nil {
		t.Fatalf("expected proxy alias + socks URL to parse: %v", err)
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

func TestParseCLAUDE_CODE_DISABLE_MOUSE(t *testing.T) {
	isolateHome(t)
	configPath := writeConfig(t, `
provider: openai
model: yaml-model
api_key: yaml-key
base_url: https://yaml.example/v1
`)
	t.Setenv("CLAUDE_CODE_DISABLE_MOUSE", "1")
	cfg, err := Parse([]string{"run", "--config", configPath, "hello"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !cfg.DisableMouse {
		t.Fatalf("expected DisableMouse when CLAUDE_CODE_DISABLE_MOUSE=1")
	}

	t.Setenv("CLAUDE_CODE_DISABLE_MOUSE", "")
	cfg2, err := Parse([]string{"run", "--config", configPath, "hello"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg2.DisableMouse {
		t.Fatalf("expected DisableMouse false when env unset")
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
grep: false
glob: false
task_graph: false
`)

	cfg, err := Parse([]string{"run", "--config", configPath, "hello"})
	if err != nil {
		t.Fatalf("expected config file, got %v", err)
	}
	if got := len(cfg.AgentConfig.Tools); got != 0 {
		t.Fatalf("expected no child tools, got %d tools", got)
	}
}

func TestParseSkillsDirLoadsLoadSkillTool(t *testing.T) {
	isolateHome(t)
	skillsRoot := t.TempDir()
	skillDir := filepath.Join(skillsRoot, "demo")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	skillFile := filepath.Join(skillDir, "SKILL.md")
	fm := "---\nname: demo-skill\ndescription: test skill\n---\n\nBody.\n"
	if err := os.WriteFile(skillFile, []byte(fm), 0644); err != nil {
		t.Fatal(err)
	}

	configPath := writeConfig(t, `
provider: anthropic
model: yaml-claude
api_key: yaml-key
`)

	cfg, err := Parse([]string{"run", "--config", configPath, "--skills-dir", skillsRoot, "hello"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var names []string
	for _, tool := range cfg.AgentConfig.Tools {
		names = append(names, tool.Name)
	}
	var hasLoadSkill bool
	for _, n := range names {
		if n == "load_skill" {
			hasLoadSkill = true
			break
		}
	}
	if !hasLoadSkill {
		t.Fatalf("expected load_skill tool, got %#v", names)
	}
	if !strings.Contains(cfg.AgentConfig.SystemPrompt, "Skills available:") {
		t.Fatalf("expected skills list in system prompt, got:\n%s", cfg.AgentConfig.SystemPrompt)
	}
	if !strings.Contains(cfg.AgentConfig.SystemPrompt, "demo-skill: test skill") {
		t.Fatalf("expected skill description in system prompt")
	}
}

func TestParseDefaultEnnoSkillsDir(t *testing.T) {
	home := isolateHome(t)
	ennoSkills := filepath.Join(home, ".enno", "skills", "acme")
	if err := os.MkdirAll(ennoSkills, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ennoSkills, "SKILL.md"), []byte("---\nname: home-skill\ndescription: from default dir\n---\n\nx\n"), 0644); err != nil {
		t.Fatal(err)
	}

	configPath := writeConfig(t, `
provider: anthropic
model: yaml-claude
api_key: yaml-key
`)

	cfg, err := Parse([]string{"run", "--config", configPath, "hello"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var hasLoad bool
	for _, tool := range cfg.AgentConfig.Tools {
		if tool.Name == "load_skill" {
			hasLoad = true
			break
		}
	}
	if !hasLoad {
		t.Fatal("expected load_skill from default ~/.enno/skills")
	}
	if !strings.Contains(cfg.AgentConfig.SystemPrompt, "home-skill: from default dir") {
		t.Fatalf("system prompt missing default skill: %s", cfg.AgentConfig.SystemPrompt)
	}
}

func TestParseSubagentEnablesTaskTool(t *testing.T) {
	isolateHome(t)
	configPath := writeConfig(t, `
provider: anthropic
model: yaml-claude
api_key: yaml-key
subagent: true
shell: true
filesystem: false
grep: false
glob: false
task_graph: false
`)

	cfg, err := Parse([]string{"run", "--config", configPath, "hello"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(cfg.AgentConfig.Tools) != 2 {
		t.Fatalf("expected bash + subagent, got %d tools", len(cfg.AgentConfig.Tools))
	}
	if cfg.AgentConfig.Tools[0].Name != "bash" || cfg.AgentConfig.Tools[1].Name != "subagent" {
		t.Fatalf("unexpected tools: %q, %q", cfg.AgentConfig.Tools[0].Name, cfg.AgentConfig.Tools[1].Name)
	}
}

func TestParseGrepDisabledViaYAML(t *testing.T) {
	isolateHome(t)
	configPath := writeConfig(t, `
provider: anthropic
model: yaml-claude
api_key: yaml-key
grep: false
`)

	cfg, err := Parse([]string{"run", "--config", configPath, "hello"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, tool := range cfg.AgentConfig.Tools {
		if tool.Name == grep.ToolName {
			t.Fatalf("expected grep tool omitted")
		}
	}
}

func TestParseGlobDisabledViaYAML(t *testing.T) {
	isolateHome(t)
	configPath := writeConfig(t, `
provider: anthropic
model: yaml-claude
api_key: yaml-key
glob: false
`)

	cfg, err := Parse([]string{"run", "--config", configPath, "hello"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, tool := range cfg.AgentConfig.Tools {
		if tool.Name == glob.ToolName {
			t.Fatalf("expected glob tool omitted")
		}
	}
}

func TestParseTaskGraphDisabledViaYAML(t *testing.T) {
	isolateHome(t)
	configPath := writeConfig(t, `
provider: anthropic
model: yaml-claude
api_key: yaml-key
task_graph: false
`)

	cfg, err := Parse([]string{"run", "--config", configPath, "hello"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, tool := range cfg.AgentConfig.Tools {
		if tool.Name == taskgraph.ToolCreate || tool.Name == taskgraph.ToolUpdate {
			t.Fatalf("expected task graph tools omitted, saw %q", tool.Name)
		}
	}
}

var uuidV4 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestNewSessionIDIsUUIDv4(t *testing.T) {
	for i := 0; i < 32; i++ {
		id := newSessionID()
		if id == "" || !uuidV4.MatchString(id) {
			t.Fatalf("newSessionID() = %q, want non-empty RFC 4122 UUID v4", id)
		}
	}
}

func TestParseSessionIDAndTaskGraphPrompt(t *testing.T) {
	isolateHome(t)
	configPath := writeConfig(t, `
provider: anthropic
model: yaml-claude
api_key: yaml-key
`)

	cfg, err := Parse([]string{"run", "--config", configPath, "hello"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg.SessionID == "" || !uuidV4.MatchString(cfg.SessionID) {
		t.Fatalf("SessionID = %q, want UUID v4", cfg.SessionID)
	}
	if !strings.Contains(cfg.AgentConfig.SystemPrompt, "~/.enno/tasks/"+cfg.SessionID+"/") {
		t.Fatalf("system prompt should reference session task dir, got:\n%s", cfg.AgentConfig.SystemPrompt)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	wantDir, err := filepath.Abs(filepath.Join(home, ".enno", "tasks", cfg.SessionID))
	if err != nil {
		t.Fatal(err)
	}
	gotDir, err := sessionTasksDir(cfg.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if gotDir != wantDir {
		t.Fatalf("sessionTasksDir(SessionID) = %q, want %q", gotDir, wantDir)
	}
	var sawCreate bool
	for _, tool := range cfg.AgentConfig.Tools {
		if tool.Name == taskgraph.ToolCreate {
			sawCreate = true
			break
		}
	}
	if !sawCreate {
		t.Fatal("expected task_create when task graph is enabled")
	}
}

func TestParseTaskGraphOffStillSetsUUIDSessionID(t *testing.T) {
	isolateHome(t)
	configPath := writeConfig(t, `
provider: anthropic
model: yaml-claude
api_key: yaml-key
task_graph: false
`)

	cfg, err := Parse([]string{"run", "--config", configPath, "hello"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg.SessionID == "" || !uuidV4.MatchString(cfg.SessionID) {
		t.Fatalf("SessionID = %q when task graph disabled", cfg.SessionID)
	}
	if strings.Contains(cfg.AgentConfig.SystemPrompt, "~/.enno/tasks/") {
		t.Fatal("system prompt should not mention ~/.enno/tasks/ when task graph is off")
	}
}

func TestParseCompactionExtendedYAML(t *testing.T) {
	isolateHome(t)
	configPath := writeConfig(t, `
provider: anthropic
model: yaml-claude
api_key: yaml-key
shell: false
filesystem: false
compaction:
  enabled: true
  model_context_tokens: 200000
  auto_compact_buffer_tokens: 10000
  micro_compact_tool_names:
    - bash
    - read_file
  skip_on_summarize_error: true
`)

	cfg, err := Parse([]string{"run", "--config", configPath, "hello"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	co := cfg.AgentConfig.Compaction
	if co == nil || !co.Enabled {
		t.Fatal("expected compaction enabled")
	}
	if co.ModelContextTokens != 200000 || co.AutoCompactBufferTokens != 10000 {
		t.Fatalf("model window fields: %#v", co)
	}
	if len(co.MicroCompactToolNames) != 2 || co.MicroCompactToolNames[0] != "bash" {
		t.Fatalf("micro names: %#v", co.MicroCompactToolNames)
	}
	if !co.SkipOnSummarizeError {
		t.Fatal("expected skip_on_summarize_error")
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
