package enno

import (
	"context"
	"fmt"
	"sync"
)

type Agent struct {
	provider      Provider
	systemPrompt  string
	tools         []Tool
	toolMap       map[string]Tool
	maxToolRounds int

	mu      sync.Mutex
	history []Message
}

func NewAgent(config Config) (*Agent, error) {
	config = config.withDefaults()
	if config.Provider == nil {
		return nil, ErrMissingProvider
	}
	tools := append([]Tool(nil), config.Tools...)
	return &Agent{
		provider:      config.Provider,
		systemPrompt:  config.SystemPrompt,
		tools:         tools,
		toolMap:       ToolMap(tools),
		maxToolRounds: config.MaxToolRounds,
	}, nil
}

func (a *Agent) Run(ctx context.Context, input string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.history = append(a.history, UserMessage(input))
	return a.runLocked(ctx)
}

func (a *Agent) Messages() []Message {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]Message(nil), a.history...)
}

func (a *Agent) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.history = nil
}

func (a *Agent) runLocked(ctx context.Context) (string, error) {
	roundsSinceTodo := 0
	for round := 0; round < a.maxToolRounds; round++ {
		resp, err := a.provider.Complete(ctx, Request{
			SystemPrompt: a.systemPrompt,
			Messages:     append([]Message(nil), a.history...),
			Tools:        append([]Tool(nil), a.tools...),
		})
		if err != nil {
			return "", err
		}

		a.history = append(a.history, AssistantMessage(resp.Content, resp.ToolCalls))
		if len(resp.ToolCalls) == 0 {
			return resp.Content, nil
		}

		usedTodo := false
		for _, toolCall := range resp.ToolCalls {
			name, result := a.executeTool(ctx, toolCall)
			if name == "todo" {
				usedTodo = true
			}
			a.history = append(a.history, ToolMessage(toolCall.ID, result))
		}
		if usedTodo {
			roundsSinceTodo = 0
		} else {
			roundsSinceTodo++
		}
		if roundsSinceTodo >= 3 {
			a.history = append(a.history, UserMessage("<reminder>Update your todos.</reminder>"))
		}
	}
	return "", fmt.Errorf("agent exceeded max tool rounds: %d", a.maxToolRounds)
}

func (a *Agent) executeTool(ctx context.Context, toolCall ToolCall) (string, string) {
	tool, ok := a.toolMap[toolCall.Name]
	if !ok {
		return toolCall.Name, fmt.Sprintf("Unknown tool: %s", toolCall.Name)
	}
	if tool.Handler == nil {
		return toolCall.Name, fmt.Sprintf("Error: tool %s has no handler", toolCall.Name)
	}
	output, err := tool.Handler(ctx, toolCall.Arguments)
	if err != nil {
		return toolCall.Name, fmt.Sprintf("Error: %v", err)
	}
	if output == "" {
		output = "(no output)"
	}
	return toolCall.Name, output
}
