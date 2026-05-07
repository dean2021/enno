package cliconfig

import (
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
)

const (
	defaultProvider       = "openai"
	defaultOpenAIBaseURL  = "https://maas-coding-api.cn-huabei-1.xf-yun.com/v2"
	defaultOpenAIModel    = "astron-code-latest"
	defaultAnthropicModel = "claude-sonnet-4-5-20250929"
	defaultMaxTokens      = int64(4096)
)

type Config struct {
	AgentConfig enno.Config
	Prompt      string
	Mode        string
	Query       string
}

func Parse(args []string) (Config, error) {
	mode := "repl"
	if len(args) > 0 && args[0] == "run" {
		mode = "run"
		args = args[1:]
	}

	wd, err := os.Getwd()
	if err != nil {
		wd = "."
	}

	fs := flag.NewFlagSet("enno", flag.ContinueOnError)
	providerName := fs.String("provider", envOr("ENNO_PROVIDER", defaultProvider), "provider: openai or anthropic")
	model := fs.String("model", os.Getenv("ENNO_MODEL"), "model name")
	apiKey := fs.String("api-key", os.Getenv("ENNO_API_KEY"), "API key")
	baseURL := fs.String("base-url", envOr("ENNO_BASE_URL", defaultOpenAIBaseURL), "OpenAI-compatible base URL")
	workdir := fs.String("workdir", wd, "tool working directory")
	noShell := fs.Bool("no-shell", false, "disable shell tool")
	noFilesystem := fs.Bool("no-filesystem", false, "disable filesystem tools")
	prompt := fs.String("prompt", "\033[36menno >> \033[0m", "REPL prompt")
	maxTokens := fs.Int64("max-tokens", envInt64("ENNO_MAX_TOKENS", defaultMaxTokens), "max output tokens for Anthropic")
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	query := strings.Join(fs.Args(), " ")
	if mode == "run" && strings.TrimSpace(query) == "" {
		return Config{}, fmt.Errorf("missing prompt for run mode")
	}

	provider, selectedModel := buildProvider(*providerName, *model, *apiKey, *baseURL, *maxTokens)
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
Prefer tools over prose.`, absOrClean(*workdir)),
			Tools: tools,
		},
		Prompt: *prompt,
		Mode:   mode,
		Query:  query,
	}, ensureModel(selectedModel)
}

func buildProvider(providerName, model, apiKey, baseURL string, maxTokens int64) (enno.Provider, string) {
	switch strings.ToLower(strings.TrimSpace(providerName)) {
	case "anthropic":
		if apiKey == "" {
			apiKey = os.Getenv("ANTHROPIC_API_KEY")
		}
		if model == "" {
			model = defaultAnthropicModel
		}
		return anthropicprovider.New(anthropicprovider.Config{
			APIKey:    apiKey,
			Model:     model,
			MaxTokens: maxTokens,
		}), model
	default:
		if model == "" {
			model = defaultOpenAIModel
		}
		return openaiprovider.New(openaiprovider.Config{
			APIKey:  apiKey,
			BaseURL: baseURL,
			Model:   model,
		}), model
	}
}

func ensureModel(model string) error {
	if strings.TrimSpace(model) == "" {
		return fmt.Errorf("missing model")
	}
	return nil
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func envInt64(name string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	var parsed int64
	if _, err := fmt.Sscan(value, &parsed); err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func absOrClean(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return abs
}
