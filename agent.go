package enno

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type Agent struct {
	provider      Provider
	systemPrompt  string
	tools         []Tool
	toolMap       map[string]Tool
	maxToolRounds int
	eventHandler  EventHandler

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
		eventHandler:  config.EventHandler,
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
		roundNumber := round + 1
		req := Request{
			SystemPrompt: a.systemPrompt,
			Messages:     append([]Message(nil), a.history...),
			Tools:        append([]Tool(nil), a.tools...),
		}
		a.emit(ctx, Event{
			Type:         EventModelStart,
			Round:        roundNumber,
			MessageCount: len(req.Messages),
			ToolCount:    len(req.Tools),
			Usage:        EstimateUsage(req),
		})

		start := time.Now()
		resp, err := a.provider.Complete(ctx, req)
		if err != nil {
			a.emit(ctx, Event{
				Type:         EventError,
				Round:        roundNumber,
				MessageCount: len(req.Messages),
				ToolCount:    len(req.Tools),
				Duration:     time.Since(start),
				Err:          err,
			})
			return "", err
		}

		a.history = append(a.history, AssistantMessage(resp.Content, resp.ToolCalls))
		usage := resp.Usage.withTotal()
		if usage.InputTokens == 0 && usage.OutputTokens == 0 && usage.TotalTokens == 0 {
			usage = EstimateUsage(req)
		}
		a.emit(ctx, Event{
			Type:         EventModelResponse,
			Round:        roundNumber,
			MessageCount: len(a.history),
			ToolCount:    len(req.Tools),
			Content:      resp.Content,
			Thinking:     resp.Thinking,
			Usage:        usage,
			Duration:     time.Since(start),
		})
		if len(resp.ToolCalls) == 0 {
			a.emit(ctx, Event{
				Type:         EventRoundComplete,
				Round:        roundNumber,
				MessageCount: len(a.history),
				ToolCount:    len(req.Tools),
				Usage:        usage,
			})
			return resp.Content, nil
		}

		usedTodo := false
		for _, toolCall := range resp.ToolCalls {
			a.emit(ctx, Event{
				Type:         EventToolStart,
				Round:        roundNumber,
				MessageCount: len(a.history),
				ToolCount:    len(req.Tools),
				ToolCall:     toolCall,
			})
			toolStart := time.Now()
			_, result := a.executeTool(ctx, toolCall)
			if _, ok := a.toolMap[toolCall.Name]; ok && toolCall.Name == "todo" {
				usedTodo = true
			}
			a.history = append(a.history, ToolMessage(toolCall.ID, result))
			a.emit(ctx, Event{
				Type:         EventToolResult,
				Round:        roundNumber,
				MessageCount: len(a.history),
				ToolCount:    len(req.Tools),
				ToolCall:     toolCall,
				ToolResult:   result,
				Duration:     time.Since(toolStart),
			})
		}
		if _, hasTodo := a.toolMap["todo"]; hasTodo {
			if usedTodo {
				roundsSinceTodo = 0
			} else {
				roundsSinceTodo++
			}
			if roundsSinceTodo >= 3 {
				a.history = append(a.history, UserMessage("<reminder>Update your todos.</reminder>"))
			}
		}
		a.emit(ctx, Event{
			Type:         EventRoundComplete,
			Round:        roundNumber,
			MessageCount: len(a.history),
			ToolCount:    len(req.Tools),
			Usage:        usage,
		})
	}
	err := fmt.Errorf("agent exceeded max tool rounds: %d", a.maxToolRounds)
	a.emit(ctx, Event{Type: EventError, Err: err})
	return "", err
}

func (a *Agent) emit(ctx context.Context, event Event) {
	if a.eventHandler != nil {
		a.eventHandler(ctx, event)
	}
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
