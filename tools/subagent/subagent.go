package subagent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/dean2021/enno"
)

// DefaultToolName is the standard tool name for spawning a subagent (parent only).
const DefaultToolName = "task"

const (
	defaultMaxToolRounds  = 30
	defaultMaxResultChars = 50000
)

// DefaultSystemPrompt is used when [Config.SystemPrompt] is empty.
const DefaultSystemPrompt = `You are a focused subagent with a clean context window (no parent conversation).
Complete the delegated task using your tools. Be concise: your final reply is returned to the parent agent as the only summary—verbose logs do not carry over.`

// Config builds the task tool that runs an isolated child [enno.Agent].
type Config struct {
	Provider       enno.Provider
	ChildTools     []enno.Tool
	SystemPrompt   string
	MaxToolRounds  int
	MaxResultChars int
	ToolName       string
	EventHandler   enno.EventHandler
}

func (c *Config) withDefaults() {
	if c.ToolName == "" {
		c.ToolName = DefaultToolName
	}
	if c.SystemPrompt == "" {
		c.SystemPrompt = DefaultSystemPrompt
	}
	if c.MaxToolRounds <= 0 {
		c.MaxToolRounds = defaultMaxToolRounds
	}
	if c.MaxResultChars <= 0 {
		c.MaxResultChars = defaultMaxResultChars
	}
}

// New returns a tool that spawns a child agent with fresh history and ChildTools only (no recursive task).
func New(cfg Config) (enno.Tool, error) {
	cfg.withDefaults()
	if cfg.Provider == nil {
		return enno.Tool{}, errors.New("subagent: Provider is required")
	}
	if len(cfg.ChildTools) == 0 {
		return enno.Tool{}, errors.New("subagent: ChildTools is required")
	}
	for _, t := range cfg.ChildTools {
		if t.Name == cfg.ToolName {
			return enno.Tool{}, fmt.Errorf("subagent: ChildTools must not include %q (recursive dispatch)", cfg.ToolName)
		}
	}

	childTools := append([]enno.Tool(nil), cfg.ChildTools...)
	toolName := cfg.ToolName
	sys := cfg.SystemPrompt
	maxRounds := cfg.MaxToolRounds
	maxChars := cfg.MaxResultChars
	ev := cfg.EventHandler
	provider := cfg.Provider

	description := `Spawn a subagent with a fresh message context to handle a delegated subtask. ` +
		`The subagent sees only your prompt and its own tool results; only its final text reply is returned here—use for exploration that would clutter the main conversation.`

	return enno.NewTypedTool(toolName, description, map[string]any{
		"prompt": map[string]any{
			"type":        "string",
			"description": "Instructions for the subagent (what to find, compute, or verify).",
		},
	}, []string{"prompt"}, func(ctx context.Context, args struct {
		Prompt string `json:"prompt"`
	}) (string, error) {
		if strings.TrimSpace(args.Prompt) == "" {
			return "", errors.New("prompt is required")
		}
		child, err := enno.NewAgent(enno.Config{
			Provider:      provider,
			SystemPrompt:  sys,
			Tools:         childTools,
			MaxToolRounds: maxRounds,
			EventHandler:  ev,
		})
		if err != nil {
			return "", err
		}
		out, err := child.Run(ctx, args.Prompt)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(out) == "" {
			out = "(no summary)"
		}
		return truncateUTF8(out, maxChars), nil
	}), nil
}

func truncateUTF8(s string, maxBytes int) string {
	if maxBytes <= 0 || len(s) <= maxBytes {
		return s
	}
	const suffix = "\n\n[truncated]"
	budget := maxBytes - len(suffix)
	if budget <= 0 {
		// Degenerate: return empty or best-effort prefix
		return trimBrokenTail(s[:maxBytes])
	}
	raw := s[:budget]
	raw = trimBrokenTail(raw)
	return raw + suffix
}

func trimBrokenTail(s string) string {
	for len(s) > 0 && !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}
