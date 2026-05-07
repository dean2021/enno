package cliconfig

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dean2021/enno"
	anthropicprovider "github.com/dean2021/enno/provider/anthropic"
	openaiprovider "github.com/dean2021/enno/provider/openai"
	compacttool "github.com/dean2021/enno/tools/compact"
	"github.com/dean2021/enno/tools/filesystem"
	"github.com/dean2021/enno/tools/glob"
	"github.com/dean2021/enno/tools/grep"
	"github.com/dean2021/enno/tools/loadskill"
	"github.com/dean2021/enno/tools/shell"
	"github.com/dean2021/enno/tools/subagent"
	"github.com/dean2021/enno/tools/taskgraph"
	"gopkg.in/yaml.v3"
)

const (
	defaultProvider  = "openai"
	defaultMaxTokens = int64(4096)
)

const defaultConfigTemplate = `# Enno CLI configuration.
# Uncomment and fill the fields you need.
#
# OpenAI-compatible example:
# provider: openai
# model: your-model
# api_key: your-key
# base_url: https://example.com/v1
#
# Anthropic example:
# provider: anthropic
# model: claude-sonnet-4-5-20250929
# api_key: your-anthropic-key
#
# max_tokens: 4096
# shell: true
# filesystem: true
#
# Optional: enable the task tool (subagent with isolated context). Default is off.
# subagent: true
#
# Default skills directory is ~/.enno/skills (created by you; missing dirs are ignored).
# Optional: extra directories (merged; later paths override same skill name).
# skills_extra_dirs:
#   - ~/Projects/shared-skills
#
# Optional legacy single extra path (same merge rules).
# skills_dir: /path/to/more
#
# Context compaction: on by default (registers compact; micro runs while enabled; full summarization only when
# over the token threshold or the model calls compact— not every turn). Set enabled: false to disable.
compaction:
  enabled: true
  transcript_dir: ~/.enno/transcripts
  auto_compact_input_tokens: 50000
  keep_recent_tool_results: 3
  micro_compact_min_chars: 100
  skip_on_summarize_error: false
# Optional tuning:
#   model_context_tokens: 200000
#   auto_compact_buffer_tokens: 13000
#   micro_compact_tool_names: [bash, read_file]
#
# grep: false   # disable ripgrep-backed Grep tool (default on when omitted)
# glob: false   # disable ripgrep-backed Glob tool (default on when omitted)
# task_graph: false   # disable persistent task graph (task_create/update/list/get; default on when omitted)
`

type Config struct {
	AgentConfig enno.Config
	Prompt      string
	Mode        string
	Query       string
	Project     string
	SessionID   string
}

type fileConfig struct {
	Provider        string           `yaml:"provider"`
	Model           string           `yaml:"model"`
	APIKey          string           `yaml:"api_key"`
	BaseURL         string           `yaml:"base_url"`
	MaxTokens       int64            `yaml:"max_tokens"`
	Shell           *bool            `yaml:"shell"`
	Filesystem      *bool            `yaml:"filesystem"`
	Subagent        *bool            `yaml:"subagent"`
	Grep            *bool            `yaml:"grep"`
	Glob            *bool            `yaml:"glob"`
	TaskGraph       *bool            `yaml:"task_graph"`
	SkillsDir       string           `yaml:"skills_dir"`
	SkillsExtraDirs []string         `yaml:"skills_extra_dirs"`
	Compaction      *compactionField `yaml:"compaction"`
}

// compactionField unmarshals either compaction: true or a mapping (see UnmarshalYAML).
type compactionField struct {
	Value *enno.CompactionConfig
}

func (c *compactionField) UnmarshalYAML(n *yaml.Node) error {
	switch n.Kind {
	case yaml.ScalarNode:
		var b bool
		if err := n.Decode(&b); err != nil {
			return err
		}
		if b {
			c.Value = &enno.CompactionConfig{Enabled: true}
		}
		return nil
	case yaml.MappingNode:
		var raw struct {
			Enabled                 bool     `yaml:"enabled"`
			TranscriptDir           string   `yaml:"transcript_dir"`
			ModelContextTokens      int64    `yaml:"model_context_tokens"`
			AutoCompactBufferTokens int64    `yaml:"auto_compact_buffer_tokens"`
			AutoCompactInputTokens  int64    `yaml:"auto_compact_input_tokens"`
			KeepRecentToolResults   int      `yaml:"keep_recent_tool_results"`
			MicroCompactMinChars    int      `yaml:"micro_compact_min_chars"`
			MicroCompactToolNames   []string `yaml:"micro_compact_tool_names"`
			SkipOnSummarizeError    bool     `yaml:"skip_on_summarize_error"`
		}
		if err := n.Decode(&raw); err != nil {
			return err
		}
		c.Value = &enno.CompactionConfig{
			Enabled:                 raw.Enabled,
			TranscriptDir:           strings.TrimSpace(raw.TranscriptDir),
			ModelContextTokens:      raw.ModelContextTokens,
			AutoCompactBufferTokens: raw.AutoCompactBufferTokens,
			AutoCompactInputTokens:  raw.AutoCompactInputTokens,
			KeepRecentToolResults:   raw.KeepRecentToolResults,
			MicroCompactMinChars:    raw.MicroCompactMinChars,
			MicroCompactToolNames:   append([]string(nil), raw.MicroCompactToolNames...),
			SkipOnSummarizeError:    raw.SkipOnSummarizeError,
		}
		return nil
	default:
		return fmt.Errorf("compaction: expected bool or mapping")
	}
}

func Parse(args []string) (Config, error) {
	mode, args := splitMode(args)
	configPath, explicitConfig, err := preparseConfigPath(args)
	if err != nil {
		return Config{}, err
	}
	fileCfg, err := loadFileConfig(configPath, explicitConfig)
	if err != nil {
		return Config{}, err
	}

	wd, err := os.Getwd()
	if err != nil {
		wd = "."
	}
	noShellDefault := !boolDefault(fileCfg.Shell, true)
	noFilesystemDefault := !boolDefault(fileCfg.Filesystem, true)
	noSubagentDefault := !boolDefault(fileCfg.Subagent, false)
	noGrepDefault := !boolDefault(fileCfg.Grep, true)
	noGlobDefault := !boolDefault(fileCfg.Glob, true)
	noTaskGraphDefault := !boolDefault(fileCfg.TaskGraph, true)

	fs := flag.NewFlagSet("enno", flag.ContinueOnError)
	fs.String("config", configPath, "config file path")
	workdir := fs.String("workdir", wd, "tool working directory")
	noShell := fs.Bool("no-shell", noShellDefault, "disable shell tool")
	noFilesystem := fs.Bool("no-filesystem", noFilesystemDefault, "disable filesystem tools")
	noSubagent := fs.Bool("no-subagent", noSubagentDefault, "disable task (subagent) tool")
	noGrep := fs.Bool("no-grep", noGrepDefault, "disable Grep (ripgrep) search tool")
	noGlob := fs.Bool("no-glob", noGlobDefault, "disable Glob (ripgrep file listing) tool")
	noTaskGraph := fs.Bool("no-task-graph", noTaskGraphDefault, "disable persistent task graph tools (task_create, task_update, task_list, task_get)")
	skillsDirFlag := fs.String("skills-dir", "", "extra SKILL.md directory merged after defaults and config (see skills_extra_dirs)")
	prompt := fs.String("prompt", "\033[36menno >> \033[0m", "REPL prompt")
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	query := strings.Join(fs.Args(), " ")
	if mode == "run" && strings.TrimSpace(query) == "" {
		return Config{}, fmt.Errorf("missing prompt for run mode")
	}

	provider, err := buildProvider(fileCfg)
	if err != nil {
		return Config{}, err
	}

	sessionID := newSessionID()
	for i := 0; i < 3 && sessionID == ""; i++ {
		sessionID = newSessionID()
	}
	if sessionID == "" {
		return Config{}, fmt.Errorf("could not generate session id")
	}

	childTools := []enno.Tool(nil)
	if !*noTaskGraph {
		tasksDirAbs, err := sessionTasksDir(sessionID)
		if err != nil {
			return Config{}, err
		}
		childTools = append(childTools, taskgraph.New(taskgraph.Config{
			Root:     *workdir,
			TasksDir: tasksDirAbs,
			Timeout:  120 * time.Second,
		})...)
	}
	if !*noFilesystem {
		childTools = append(childTools, filesystem.New(filesystem.Config{Root: *workdir})...)
	}
	if !*noShell {
		childTools = append(childTools, shell.New(shell.Config{Workdir: *workdir, Timeout: 120 * time.Second}))
	}
	if !*noGrep {
		childTools = append(childTools, grep.New(grep.Config{Root: *workdir, Timeout: 120 * time.Second}))
	}
	if !*noGlob {
		childTools = append(childTools, glob.New(glob.Config{Root: *workdir, Timeout: 120 * time.Second}))
	}

	skillRoots, err := collectSkillRoots(fileCfg, *skillsDirFlag)
	if err != nil {
		return Config{}, err
	}
	var skillRegistry *loadskill.Registry
	if len(skillRoots) > 0 {
		reg, err := loadskill.LoadDirs(skillRoots)
		if err != nil {
			return Config{}, err
		}
		if reg.Count() > 0 {
			skillTool, err := loadskill.NewTool(reg)
			if err != nil {
				return Config{}, err
			}
			childTools = append(childTools, skillTool)
			skillRegistry = reg
		}
	}

	tools := append([]enno.Tool(nil), childTools...)

	var compaction *enno.CompactionConfig
	if fileCfg.Compaction != nil && fileCfg.Compaction.Value != nil {
		cc := *fileCfg.Compaction.Value
		if cc.Enabled && strings.TrimSpace(cc.TranscriptDir) != "" {
			ex, err := expandUserPath(cc.TranscriptDir)
			if err != nil {
				return Config{}, fmt.Errorf("compaction transcript_dir: %w", err)
			}
			cc.TranscriptDir = ex
		}
		compaction = &cc
	}
	if compaction != nil && compaction.Enabled {
		tools = append(tools, compacttool.New())
	}

	if !*noSubagent {
		if len(childTools) == 0 {
			return Config{}, fmt.Errorf("subagent enabled but no child tools: enable at least one of task_graph, filesystem, shell, grep, glob, or skills")
		}
		taskTool, err := subagent.New(subagent.Config{
			Provider:   provider,
			ChildTools: append([]enno.Tool(nil), childTools...),
		})
		if err != nil {
			return Config{}, err
		}
		tools = append(tools, taskTool)
	}

	sys := fmt.Sprintf(`You are a coding agent at %s.
Prefer tools over prose.`, absOrClean(*workdir))
	if !*noTaskGraph {
		sys += fmt.Sprintf(`

Use task_create, task_update, task_list, and task_get to plan and track work as a persistent task graph stored under ~/.enno/tasks/%s/ for this CLI session. Use pending / in_progress / completed; use blocked_by for dependencies. If you run several tool rounds without using any of these task tools, the runtime may insert a short reminder.`, sessionID)
	}
	if !*noGrep {
		sys += `

Use the Grep tool for searching file contents (regex via ripgrep), not grep/rg shell commands.`
	}
	if !*noGlob {
		sys += `

Use the Glob tool to find files by name/glob patterns; do not use shell find/ls for discovery when Glob suffices.`
	}
	if !*noSubagent {
		sys += `

You may use the task tool to delegate a subtask to an isolated subagent (fresh context). Only the subagent's final reply is returned—use for exploration that would clutter this conversation.`
	}
	if skillRegistry != nil {
		sys += `

Skills available:
` + skillRegistry.DescriptionsText() + `
Call load_skill with a skill name when you need the full instructions for that workflow.`
	}
	if compaction != nil && compaction.Enabled {
		sys += `

Context compaction is enabled: long contexts may be summarized automatically; you may also call the compact tool alone in one assistant turn to replace history with a compressed summary (extra model call).`
	}

	return Config{
		AgentConfig: enno.Config{
			Provider:     provider,
			SystemPrompt: sys,
			Tools:        tools,
			Compaction:   compaction,
		},
		Prompt:    *prompt,
		Mode:      mode,
		Query:     query,
		Project:   absOrClean(*workdir),
		SessionID: sessionID,
	}, nil
}

func buildProvider(config fileConfig) (enno.Provider, error) {
	providerName := strings.ToLower(strings.TrimSpace(config.Provider))
	if providerName == "" {
		providerName = defaultProvider
	}
	model := strings.TrimSpace(config.Model)
	baseURL := strings.TrimSpace(config.BaseURL)
	if model == "" {
		return nil, fmt.Errorf("missing model: set model in config.yaml")
	}

	switch providerName {
	case "anthropic":
		return anthropicprovider.New(anthropicprovider.Config{
			APIKey:    config.APIKey,
			Model:     model,
			MaxTokens: positiveOr(config.MaxTokens, defaultMaxTokens),
		}), nil
	case "openai":
		if baseURL == "" {
			return nil, fmt.Errorf("missing OpenAI-compatible base URL: set base_url in config.yaml")
		}
		return openaiprovider.New(openaiprovider.Config{
			APIKey:  config.APIKey,
			BaseURL: baseURL,
			Model:   model,
		}), nil
	default:
		return nil, fmt.Errorf("unsupported provider %q: use openai or anthropic", providerName)
	}
}

func splitMode(args []string) (string, []string) {
	if len(args) > 0 && args[0] == "run" {
		return "run", args[1:]
	}
	return "repl", args
}

func preparseConfigPath(args []string) (string, bool, error) {
	defaultPath, err := defaultConfigPath()
	if err != nil {
		return "", false, err
	}
	for i, arg := range args {
		switch {
		case arg == "--config":
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				return "", true, fmt.Errorf("missing value for --config")
			}
			return args[i+1], true, nil
		case strings.HasPrefix(arg, "--config="):
			path := strings.TrimPrefix(arg, "--config=")
			if strings.TrimSpace(path) == "" {
				return "", true, fmt.Errorf("missing value for --config")
			}
			return path, true, nil
		}
	}
	return defaultPath, false, nil
}

func defaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".enno", "config.yaml"), nil
}

func loadFileConfig(path string, explicit bool) (fileConfig, error) {
	var config fileConfig
	bytes, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) && !explicit {
			if err := createDefaultConfigFile(path); err != nil {
				return config, err
			}
			return config, nil
		}
		return config, fmt.Errorf("read config file %s: %w", path, err)
	}
	if err := yaml.Unmarshal(bytes, &config); err != nil {
		return config, fmt.Errorf("parse config file %s: %w", path, err)
	}
	return config, nil
}

func createDefaultConfigFile(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create config directory %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(defaultConfigTemplate), 0600); err != nil {
		return fmt.Errorf("create config file %s: %w", path, err)
	}
	return nil
}

func positiveOr(value, fallback int64) int64 {
	if value > 0 {
		return value
	}
	return fallback
}

func boolDefault(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func absOrClean(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return abs
}

// expandUserPath resolves ~ and returns an absolute path.
func expandUserPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
	}
	return filepath.Abs(path)
}

// defaultSkillsDir returns ~/.enno/skills (expanded).
func defaultSkillsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Abs(filepath.Join(home, ".enno", "skills"))
}

// collectSkillRoots builds the ordered list of skill directories: default ~/.enno/skills,
// then skills_extra_dirs, then legacy skills_dir, then --skills-dir. Duplicate paths are dropped.
func collectSkillRoots(fileCfg fileConfig, skillsDirFlag string) ([]string, error) {
	var raw []string
	if def, err := defaultSkillsDir(); err != nil {
		return nil, err
	} else {
		raw = append(raw, def)
	}
	raw = append(raw, fileCfg.SkillsExtraDirs...)
	if s := strings.TrimSpace(fileCfg.SkillsDir); s != "" {
		raw = append(raw, s)
	}
	if s := strings.TrimSpace(skillsDirFlag); s != "" {
		raw = append(raw, s)
	}

	seen := make(map[string]bool)
	var out []string
	for _, r := range raw {
		ex, err := expandUserPath(strings.TrimSpace(r))
		if err != nil {
			return nil, fmt.Errorf("skills directory: %w", err)
		}
		if ex == "" {
			continue
		}
		if seen[ex] {
			continue
		}
		seen[ex] = true
		out = append(out, ex)
	}
	return out, nil
}

// newSessionID returns an RFC 4122 UUID v4 string (same role as Node crypto.randomUUID() in Claude Code).
func newSessionID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant RFC 4122
	h := hex.EncodeToString(b)
	return fmt.Sprintf("%s-%s-%s-%s-%s", h[0:8], h[8:12], h[12:16], h[16:20], h[20:32])
}

// sessionTasksDir returns the absolute path ~/.enno/tasks/<sessionID>/ for CLI task graph storage.
func sessionTasksDir(sessionID string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	p := filepath.Join(home, ".enno", "tasks", sessionID)
	return filepath.Abs(p)
}
