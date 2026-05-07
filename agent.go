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
	compaction    *CompactionConfig

	// Last successful Complete response input tokens from the provider (when > 0).
	lastCompleteInputTokens int64
	compactionFailStreak    int

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
		compaction:    config.Compaction,
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
	a.lastCompleteInputTokens = 0
	a.compactionFailStreak = 0
}

func (a *Agent) runLocked(ctx context.Context) (string, error) {
	a.compactionFailStreak = 0
	roundsSincePlan := 0
	for round := 0; round < a.maxToolRounds; round++ {
		roundNumber := round + 1
		if err := a.maybeAutoCompact(ctx, roundNumber); err != nil {
			a.emit(ctx, Event{
				Type:         EventError,
				Round:        roundNumber,
				MessageCount: len(a.history),
				ToolCount:    len(a.tools),
				Err:          err,
			})
			return "", err
		}
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
		if resp.Usage.InputTokens > 0 {
			a.lastCompleteInputTokens = resp.Usage.InputTokens
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

		if a.compaction != nil && a.compaction.Enabled && len(resp.ToolCalls) == 1 && resp.ToolCalls[0].Name == CompactionToolName {
			if _, ok := a.toolMap[CompactionToolName]; ok {
				assistantMsg := a.history[len(a.history)-1]
				a.history = a.history[:len(a.history)-1]
				transcript := append(append([]Message(nil), a.history...), assistantMsg)
				toolCall := resp.ToolCalls[0]
				a.emit(ctx, Event{
					Type:         EventToolStart,
					Round:        roundNumber,
					MessageCount: len(a.history),
					ToolCount:    len(req.Tools),
					ToolCall:     toolCall,
				})
				toolStart := time.Now()
				result, err := a.runCompactionSummarize(ctx, transcript)
				if err != nil {
					a.history = append(a.history, assistantMsg)
					a.emit(ctx, Event{
						Type:         EventError,
						Round:        roundNumber,
						MessageCount: len(a.history),
						ToolCount:    len(req.Tools),
						Err:          err,
					})
					return "", err
				}
				a.emit(ctx, Event{
					Type:         EventToolResult,
					Round:        roundNumber,
					MessageCount: len(a.history),
					ToolCount:    len(req.Tools),
					ToolCall:     toolCall,
					ToolResult:   result,
					Duration:     time.Since(toolStart),
				})
				a.emit(ctx, Event{
					Type:         EventRoundComplete,
					Round:        roundNumber,
					MessageCount: len(a.history),
					ToolCount:    len(req.Tools),
					Usage:        usage,
				})
				continue
			}
		}

		usedPlanTool := false
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
			if _, ok := a.toolMap[toolCall.Name]; ok && isTaskGraphToolName(toolCall.Name) {
				usedPlanTool = true
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
		if a.hasTaskGraphTools() {
			if usedPlanTool {
				roundsSincePlan = 0
			} else {
				roundsSincePlan++
			}
			if roundsSincePlan >= 3 {
				a.history = append(a.history, UserMessage("<reminder>Update your task plan.</reminder>"))
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
	if a.compaction != nil && a.compaction.Enabled && toolCall.Name == CompactionToolName {
		if _, ok := a.toolMap[CompactionToolName]; ok {
			return toolCall.Name, "compact must be the only tool call in the assistant message"
		}
	}
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

func (a *Agent) maybeAutoCompact(ctx context.Context, roundNumber int) error {
	if a.compaction == nil || !a.compaction.Enabled {
		return nil
	}
	if a.compactionFailStreak >= maxConsecutiveCompactionFailures {
		return nil
	}
	cfg := *a.compaction
	microCompact(a.history, cfg.KeepRecentToolResults, cfg.MicroCompactMinChars, cfg.MicroCompactToolNames)
	req := Request{
		SystemPrompt: a.systemPrompt,
		Messages:     a.history,
		Tools:        a.tools,
	}
	if !inputTokensOverThreshold(req, cfg, a.lastCompleteInputTokens) {
		return nil
	}
	if _, err := saveCompactionTranscript(cfg.TranscriptDir, a.history); err != nil {
		return err
	}
	summary, err := summarizeCompaction(ctx, a.provider, append([]Message(nil), a.history...))
	if err != nil {
		a.compactionFailStreak++
		if cfg.SkipOnSummarizeError {
			a.emit(ctx, Event{
				Type:         EventError,
				Round:        roundNumber,
				MessageCount: len(a.history),
				ToolCount:    len(a.tools),
				Err:          err,
			})
			return nil
		}
		return err
	}
	a.compactionFailStreak = 0
	a.history = []Message{UserMessage(compressedUserContent(summary))}
	return nil
}

func (a *Agent) runCompactionSummarize(ctx context.Context, transcript []Message) (string, error) {
	if _, err := saveCompactionTranscript(a.compaction.TranscriptDir, transcript); err != nil {
		return "", err
	}
	summary, err := summarizeCompaction(ctx, a.provider, transcript)
	if err != nil {
		return "", err
	}
	a.history = []Message{UserMessage(compressedUserContent(summary))}
	return "Compaction completed.", nil
}

// task graph tool names (tools/taskgraph); keep in sync without importing tools/* from root.
var taskGraphToolNames = []string{"task_create", "task_update", "task_list", "task_get"}

func (a *Agent) hasTaskGraphTools() bool {
	for _, n := range taskGraphToolNames {
		if _, ok := a.toolMap[n]; ok {
			return true
		}
	}
	return false
}

func isTaskGraphToolName(name string) bool {
	for _, n := range taskGraphToolNames {
		if name == n {
			return true
		}
	}
	return false
}
