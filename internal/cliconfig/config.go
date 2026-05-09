package cliconfig

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dean2021/enno"
	"github.com/dean2021/enno/internal/cliprompt"
	"github.com/dean2021/enno/internal/projectrules"
	anthropicprovider "github.com/dean2021/enno/provider/anthropic"
	openaiprovider "github.com/dean2021/enno/provider/openai"
	"github.com/dean2021/enno/sdk"
	"gopkg.in/yaml.v3"
)

const (
	defaultProvider  = "openai"
	defaultMaxTokens = int64(4096)
)

const defaultIdentityTemplate = `You are Enno, an interactive coding agent running at %s.

Use the instructions below and the tools available to you to assist the user with software engineering tasks. Prefer tools over prose — when you can act, act.`

const defaultConfigTemplate = `# Enno CLI — config path: ~/.enno/config.yaml (override with --config).
# Uncomment one provider block and set api_key / model (and base_url for OpenAI-compatible APIs).
#
# --- OpenAI-compatible (Chat Completions) ---
# provider: openai
# model: your-model-id
# api_key: your-key
# base_url: https://your-gateway.example/v1
#
# --- Anthropic (Messages API) ---
# provider: anthropic
# model: claude-sonnet-4-5-20250929
# api_key: your-anthropic-key
# max_tokens: 4096
#
# --- Optional (all providers) ---
# http_proxy: http://127.0.0.1:7890   # or socks5://127.0.0.1:7891; alias key: proxy
# max_http_retries: 8   # 429 / 5xx / timeout retries (SDK backoff); default 6 if omitted
# shell: true
# filesystem: true
# subagent: true        # isolated child agent tool (default off)
# fetch_url: true       # fetch HTTP/HTTPS URLs and convert HTML to markdown
#
# Prompt context: CLI auto-loads project rules from --workdir upward
# (AGENTS.md preferred, CLAUDE.md fallback), plus best-effort environment and git snapshot sections.
#
# Skills: default ~/.enno/skills; merge extras (later dirs override same skill name):
# skills_extra_dirs:
#   - ~/Projects/shared-skills
# skills_dir: /path/to/more   # single extra dir (legacy)
#
# Context compaction (below): micro-trim + optional auto summarization; disable with compaction.enabled: false
compaction:
  enabled: true
  transcript_dir: ~/.enno/transcripts
  auto_compact_input_tokens: 50000
  keep_recent_tool_results: 3
  micro_compact_min_chars: 100
  skip_on_summarize_error: false
# Optional compaction tuning:
#   model_context_tokens: 200000
#   auto_compact_buffer_tokens: 13000
#   micro_compact_tool_names: [bash, read_file]
#
# grep: false       # off: disable grep tool (default on)
# glob: false       # off: disable glob tool (default on)
# fetch_url: false  # off: disable URL fetch tool (default on)
# task_graph: false # off: disable task_create/update/list/get (default on)
# permission_mode: allow # allow, deny, or ask
# allowed_tools: []
# disallowed_tools: []
`

type Config struct {
	AgentConfig sdk.Config
	Prompt      string
	Mode        string
	Query       string
	Project     string
	SessionID   string
	// DisableMouse matches Claude Code: set CLAUDE_CODE_DISABLE_MOUSE=1 to skip TUI mouse capture (alternate screen unchanged; use keyboard to scroll).
	DisableMouse bool
}

type fileConfig struct {
	Provider       string `yaml:"provider"`
	Model          string `yaml:"model"`
	APIKey         string `yaml:"api_key"`
	BaseURL        string `yaml:"base_url"`
	MaxTokens      int64  `yaml:"max_tokens"`
	MaxHTTPRetries int    `yaml:"max_http_retries,omitempty"`
	HTTPProxy      string `yaml:"http_proxy,omitempty"`
	// Proxy is a YAML alias for http_proxy (common mistake when editing config).
	Proxy           string           `yaml:"proxy,omitempty"`
	Shell           *bool            `yaml:"shell"`
	Filesystem      *bool            `yaml:"filesystem"`
	Subagent        *bool            `yaml:"subagent"`
	Grep            *bool            `yaml:"grep"`
	Glob            *bool            `yaml:"glob"`
	FetchURL        *bool            `yaml:"fetch_url"`
	TaskGraph       *bool            `yaml:"task_graph"`
	SkillsDir       string           `yaml:"skills_dir"`
	SkillsExtraDirs []string         `yaml:"skills_extra_dirs"`
	Compaction      *compactionField `yaml:"compaction"`
	PermissionMode  string           `yaml:"permission_mode"`
	AllowedTools    []string         `yaml:"allowed_tools"`
	DisallowedTools []string         `yaml:"disallowed_tools"`
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
	noFetchURLDefault := !boolDefault(fileCfg.FetchURL, true)
	noTaskGraphDefault := !boolDefault(fileCfg.TaskGraph, true)

	fs := flag.NewFlagSet("enno", flag.ContinueOnError)
	fs.String("config", configPath, "config file path")
	workdir := fs.String("workdir", wd, "tool working directory")
	noShell := fs.Bool("no-shell", noShellDefault, "disable shell tool")
	noFilesystem := fs.Bool("no-filesystem", noFilesystemDefault, "disable filesystem tools")
	noSubagent := fs.Bool("no-subagent", noSubagentDefault, "disable subagent tool")
	noGrep := fs.Bool("no-grep", noGrepDefault, "disable grep (ripgrep) search tool")
	noGlob := fs.Bool("no-glob", noGlobDefault, "disable glob (ripgrep file listing) tool")
	noFetchURL := fs.Bool("no-fetch-url", noFetchURLDefault, "disable fetch_url HTTP/HTTPS page fetch tool")
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

	builtinTools := sdk.BuiltinTools{}
	if !*noTaskGraph {
		tasksDirAbs, err := sessionTasksDir(sessionID)
		if err != nil {
			return Config{}, err
		}
		builtinTools.TaskGraph = &sdk.TaskGraphTool{
			Root:     *workdir,
			TasksDir: tasksDirAbs,
			Timeout:  120 * time.Second,
		}
	}
	if !*noFilesystem {
		builtinTools.Filesystem = &sdk.FilesystemTool{Root: *workdir}
	}
	if !*noShell {
		builtinTools.Shell = &sdk.ShellTool{Workdir: *workdir, Timeout: 120 * time.Second}
	}
	if !*noGrep {
		builtinTools.Grep = &sdk.GrepTool{Root: *workdir, Timeout: 120 * time.Second}
	}
	if !*noGlob {
		builtinTools.Glob = &sdk.GlobTool{Root: *workdir, Timeout: 120 * time.Second}
	}
	if !*noFetchURL {
		builtinTools.FetchURL = &sdk.FetchURLTool{Timeout: 30 * time.Second}
	}

	skillRoots, err := collectSkillRoots(fileCfg, *skillsDirFlag)
	if err != nil {
		return Config{}, err
	}
	if len(skillRoots) > 0 {
		builtinTools.LoadSkill = &sdk.LoadSkillTool{Dirs: skillRoots}
	}

	var compaction *enno.CompactionConfig
	if fileCfg.Compaction != nil && fileCfg.Compaction.Value != nil {
		cc := *fileCfg.Compaction.Value
		if cc.Enabled && strings.TrimSpace(cc.TranscriptDir) == "" {
			dir, err := defaultTranscriptDir()
			if err != nil {
				return Config{}, fmt.Errorf("compaction transcript_dir: %w", err)
			}
			cc.TranscriptDir = dir
		} else if cc.Enabled && strings.TrimSpace(cc.TranscriptDir) != "" {
			ex, err := expandUserPath(cc.TranscriptDir)
			if err != nil {
				return Config{}, fmt.Errorf("compaction transcript_dir: %w", err)
			}
			cc.TranscriptDir = ex
		}
		compaction = &cc
	}

	if !*noSubagent {
		if !hasChildToolConfig(builtinTools) {
			return Config{}, fmt.Errorf("subagent tool enabled but no child tools: enable at least one of task_graph, filesystem, shell, grep, glob, fetch_url, or skills")
		}
		builtinTools.Subagent = &sdk.SubagentTool{}
	}

	projectInstructions, err := projectrules.Load(projectrules.Config{Workdir: *workdir})
	if err != nil {
		return Config{}, fmt.Errorf("load project instructions: %w", err)
	}
	envInfo := cliprompt.EnvironmentFromWorkdir(absOrClean(*workdir), time.Now())
	gitSnapshot, _ := cliprompt.LoadGitSnapshot(context.Background(), *workdir, nil, cliprompt.DefaultGitStatusLimit)
	sys := cliprompt.NewCodingAgent(cliprompt.CodingAgentConfig{
		Identity:            fmt.Sprintf(defaultIdentityTemplate, absOrClean(*workdir)),
		Environment:         &envInfo,
		GitSnapshot:         gitSnapshot,
		ProjectInstructions: toPromptInstructions(projectInstructions),
		CompactionEnabled:   compaction != nil && compaction.Enabled,
	}).Build()

	return Config{
		AgentConfig: sdk.Config{
			Provider:     provider,
			SystemPrompt: sys,
			BuiltinTools: builtinTools,
			Permissions: sdk.ToolPermissions{
				Mode:            sdk.PermissionMode(strings.TrimSpace(fileCfg.PermissionMode)),
				AllowedTools:    append([]string(nil), fileCfg.AllowedTools...),
				DisallowedTools: append([]string(nil), fileCfg.DisallowedTools...),
			},
			Compaction: compaction,
		},
		Prompt:       *prompt,
		Mode:         mode,
		Query:        query,
		Project:      absOrClean(*workdir),
		SessionID:    sessionID,
		DisableMouse: envTruthy("CLAUDE_CODE_DISABLE_MOUSE"),
	}, nil
}

// envTruthy matches Claude Code's isEnvTruthy: 1, true, yes, on (case-insensitive).
func envTruthy(key string) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch v {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func hasChildToolConfig(tools sdk.BuiltinTools) bool {
	return tools.TaskGraph != nil ||
		tools.Filesystem != nil ||
		tools.Shell != nil ||
		tools.Grep != nil ||
		tools.Glob != nil ||
		tools.FetchURL != nil ||
		tools.LoadSkill != nil
}

func toPromptInstructions(instructions []projectrules.Instruction) []cliprompt.ProjectInstruction {
	if len(instructions) == 0 {
		return nil
	}
	out := make([]cliprompt.ProjectInstruction, 0, len(instructions))
	for _, instruction := range instructions {
		out = append(out, cliprompt.ProjectInstruction{
			Path:      instruction.Path,
			Content:   instruction.Content,
			Truncated: instruction.Truncated,
		})
	}
	return out
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
		ac := anthropicprovider.Config{
			APIKey:    config.APIKey,
			Model:     model,
			MaxTokens: positiveOr(config.MaxTokens, defaultMaxTokens),
			HTTPProxy: strings.TrimSpace(config.HTTPProxy),
		}
		if config.MaxHTTPRetries > 0 {
			ac.MaxHTTPRetries = config.MaxHTTPRetries
		}
		return anthropicprovider.New(ac)
	case "openai":
		if baseURL == "" {
			return nil, fmt.Errorf("missing OpenAI-compatible base URL: set base_url in config.yaml")
		}
		oc := openaiprovider.Config{
			APIKey:    config.APIKey,
			BaseURL:   baseURL,
			Model:     model,
			HTTPProxy: strings.TrimSpace(config.HTTPProxy),
		}
		if config.MaxHTTPRetries > 0 {
			oc.MaxHTTPRetries = config.MaxHTTPRetries
		}
		return openaiprovider.New(oc)
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
	if strings.TrimSpace(config.HTTPProxy) == "" {
		config.HTTPProxy = strings.TrimSpace(config.Proxy)
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

func defaultTranscriptDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}
	return filepath.Abs(filepath.Join(home, ".enno", "transcripts"))
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
