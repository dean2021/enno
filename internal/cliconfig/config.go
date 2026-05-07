package cliconfig

import (
	"crypto/rand"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dean2021/enno"
	anthropicprovider "github.com/dean2021/enno/provider/anthropic"
	openaiprovider "github.com/dean2021/enno/provider/openai"
	"github.com/dean2021/enno/tools/filesystem"
	"github.com/dean2021/enno/tools/shell"
	"github.com/dean2021/enno/tools/todo"
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
	Provider   string `yaml:"provider"`
	Model      string `yaml:"model"`
	APIKey     string `yaml:"api_key"`
	BaseURL    string `yaml:"base_url"`
	MaxTokens  int64  `yaml:"max_tokens"`
	Shell      *bool  `yaml:"shell"`
	Filesystem *bool  `yaml:"filesystem"`
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

	fs := flag.NewFlagSet("enno", flag.ContinueOnError)
	fs.String("config", configPath, "config file path")
	workdir := fs.String("workdir", wd, "tool working directory")
	noShell := fs.Bool("no-shell", noShellDefault, "disable shell tool")
	noFilesystem := fs.Bool("no-filesystem", noFilesystemDefault, "disable filesystem tools")
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
	tools := []enno.Tool{todo.New()}
	if !*noFilesystem {
		tools = append(tools, filesystem.New(filesystem.Config{Root: *workdir})...)
	}
	if !*noShell {
		tools = append(tools, shell.New(shell.Config{Workdir: *workdir, Timeout: 120 * time.Second}))
	}

	return Config{
		AgentConfig: enno.Config{
			Provider: provider,
			SystemPrompt: fmt.Sprintf(`You are a coding agent at %s.
Use the todo tool to plan multi-step tasks. Mark in_progress before starting, completed when done.
If you run several tool rounds without updating the todo list, the runtime may insert a short reminder to refresh it.
Prefer tools over prose.`, absOrClean(*workdir)),
			Tools: tools,
		},
		Prompt:    *prompt,
		Mode:      mode,
		Query:     query,
		Project:   absOrClean(*workdir),
		SessionID: newSessionID(),
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

func newSessionID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
