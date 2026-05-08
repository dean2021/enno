package enno

import (
	"context"
	"errors"
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
	options       RequestOptions
	policies      []Policy
	hooks         []Hook

	compactionFailStreak int

	mu      sync.Mutex
	session Session
}

func NewAgent(config Config) (*Agent, error) {
	config = config.withDefaults()
	if config.Provider == nil {
		return nil, ErrMissingProvider
	}
	tools := append([]Tool(nil), config.Tools...)
	if err := validateTools(tools); err != nil {
		return nil, err
	}
	extraPolicies := append([]Policy(nil), config.Policies...)
	hooks := append([]Hook(nil), config.Hooks...)
	agent := &Agent{
		provider:      config.Provider,
		systemPrompt:  config.SystemPrompt,
		tools:         tools,
		toolMap:       ToolMap(tools),
		maxToolRounds: config.MaxToolRounds,
		eventHandler:  config.EventHandler,
		compaction:    config.Compaction,
		options:       config.Options,
		hooks:         hooks,
	}
	agent.policies = agent.defaultPolicies()
	agent.policies = append(agent.policies, extraPolicies...)
	return agent, nil
}

func (a *Agent) Run(ctx context.Context, input string) (string, error) {
	result, err := a.RunDetailed(ctx, input)
	if err != nil {
		return "", err
	}
	return result.Content, nil
}

func (a *Agent) RunDetailed(ctx context.Context, input string) (RunResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.session.Append(UserMessage(input))
	return a.runSessionLocked(ctx, &a.session, nil)
}

func (a *Agent) RunSession(ctx context.Context, session *Session, input string) (RunResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if session == nil {
		return RunResult{StopReason: StopReasonError}, ErrNilSession
	}
	session.Append(UserMessage(input))
	return a.runSessionLocked(ctx, session, nil)
}

func (a *Agent) Messages() []Message {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.session.Clone().Messages
}

func (a *Agent) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.session.Reset()
	a.compactionFailStreak = 0
}

func (a *Agent) runSessionLocked(ctx context.Context, session *Session, streamHandler StreamHandler) (RunResult, error) {
	for _, policy := range a.policies {
		if starter, ok := policy.(RunStartPolicy); ok {
			starter.OnRunStart()
		}
	}
	runStart := time.Now()
	runResult := RunResult{}
	for round := 0; a.maxToolRounds <= 0 || round < a.maxToolRounds; round++ {
		roundNumber := round + 1
		state := &RunState{
			Round:   roundNumber,
			Session: session,
		}
		if err := a.beforeModel(ctx, state); err != nil {
			a.emit(ctx, Event{
				Type:         EventError,
				Round:        roundNumber,
				MessageCount: len(session.Messages),
				ToolCount:    len(a.tools),
				Err:          err,
			})
			runResult.StopReason = stopReasonForError(err)
			runResult.Duration = time.Since(runStart)
			runResult.Messages = cloneMessages(session.Messages)
			return runResult, err
		}
		req := Request{
			SystemPrompt: a.systemPrompt,
			Messages:     cloneMessages(session.Messages),
			Tools:        append([]Tool(nil), a.tools...),
			Options:      RequestOptions{}.WithDefaults(a.options),
		}
		state.Request = req
		a.emit(ctx, Event{
			Type:         EventModelStart,
			Round:        roundNumber,
			MessageCount: len(req.Messages),
			ToolCount:    len(req.Tools),
			Usage:        EstimateUsage(req),
		})

		req, err := a.beforeProviderCallHooks(ctx, roundNumber, req)
		if err != nil {
			a.emit(ctx, Event{
				Type:         EventError,
				Round:        roundNumber,
				MessageCount: len(req.Messages),
				ToolCount:    len(req.Tools),
				Err:          err,
			})
			runResult.StopReason = stopReasonForError(err)
			runResult.Duration = time.Since(runStart)
			runResult.Messages = cloneMessages(session.Messages)
			return runResult, err
		}
		state.Request = req

		start := time.Now()
		resp, err := a.complete(ctx, req, streamHandler)
		if err != nil {
			a.emit(ctx, Event{
				Type:         EventError,
				Round:        roundNumber,
				MessageCount: len(req.Messages),
				ToolCount:    len(req.Tools),
				Duration:     time.Since(start),
				Err:          err,
			})
			runResult.StopReason = stopReasonForError(err)
			runResult.Duration = time.Since(runStart)
			runResult.Messages = cloneMessages(session.Messages)
			return runResult, err
		}
		resp, err = a.afterProviderCallHooks(ctx, roundNumber, req, resp)
		if err != nil {
			a.emit(ctx, Event{
				Type:         EventError,
				Round:        roundNumber,
				MessageCount: len(req.Messages),
				ToolCount:    len(req.Tools),
				Duration:     time.Since(start),
				Err:          err,
			})
			runResult.StopReason = stopReasonForError(err)
			runResult.Duration = time.Since(runStart)
			runResult.Messages = cloneMessages(session.Messages)
			return runResult, err
		}

		assistant := AssistantMessage(resp.Content, cloneToolCalls(resp.ToolCalls))
		session.Messages = append(session.Messages, assistant)
		usage := resp.Usage.withTotal()
		if usage.InputTokens == 0 && usage.OutputTokens == 0 && usage.TotalTokens == 0 {
			usage = EstimateUsage(req)
		}
		modelUsage := usage.withTotal()
		state.Response = resp
		state.ModelUsage = modelUsage
		addUsage(&runResult.Usage, modelUsage)
		roundResult := RoundResult{
			Round:      roundNumber,
			ModelUsage: modelUsage,
			Assistant:  cloneMessage(assistant),
			Duration:   time.Since(start),
		}
		if resp.Usage.InputTokens > 0 {
			session.lastCompleteInputTokens = resp.Usage.InputTokens
		}
		a.emit(ctx, Event{
			Type:         EventModelResponse,
			Round:        roundNumber,
			MessageCount: len(session.Messages),
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
				MessageCount: len(session.Messages),
				ToolCount:    len(req.Tools),
				Usage:        usage,
			})
			runResult.Rounds = append(runResult.Rounds, roundResult)
			runResult.Content = resp.Content
			runResult.StopReason = StopReasonEndTurn
			runResult.Duration = time.Since(runStart)
			runResult.Messages = cloneMessages(session.Messages)
			return runResult, nil
		}

		if err := a.afterModel(ctx, state); err != nil {
			runResult.StopReason = stopReasonForError(err)
			runResult.Duration = time.Since(runStart)
			runResult.Messages = cloneMessages(session.Messages)
			return runResult, err
		}
		if !state.SkipToolExecution {
			a.executeToolCalls(ctx, state)
		}
		if err := a.afterTools(ctx, state); err != nil {
			runResult.StopReason = stopReasonForError(err)
			runResult.Duration = time.Since(runStart)
			runResult.Messages = cloneMessages(session.Messages)
			return runResult, err
		}
		a.emit(ctx, Event{
			Type:         EventRoundComplete,
			Round:        roundNumber,
			MessageCount: len(session.Messages),
			ToolCount:    len(req.Tools),
			Usage:        usage,
		})
		roundResult.Duration = time.Since(start)
		roundResult.ToolCalls = append(roundResult.ToolCalls, state.ToolCallResults...)
		runResult.Rounds = append(runResult.Rounds, roundResult)
	}
	err := fmt.Errorf("agent exceeded max tool rounds: %d", a.maxToolRounds)
	a.emit(ctx, Event{Type: EventError, Err: err})
	runResult.StopReason = StopReasonMaxToolRounds
	runResult.Duration = time.Since(runStart)
	runResult.Messages = cloneMessages(session.Messages)
	return runResult, err
}

func stopReasonForError(err error) StopReason {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return StopReasonCanceled
	}
	return StopReasonError
}

func (a *Agent) defaultPolicies() []Policy {
	var policies []Policy
	if a.compaction != nil && a.compaction.Enabled {
		policies = append(policies, &compactionPolicy{agent: a})
	}
	if a.hasTaskGraphTools() {
		policies = append(policies, &taskReminderPolicy{})
	}
	return policies
}

func (a *Agent) beforeModel(ctx context.Context, state *RunState) error {
	for _, policy := range a.policies {
		if err := policy.BeforeModel(ctx, state); err != nil {
			return err
		}
	}
	return nil
}

func (a *Agent) afterModel(ctx context.Context, state *RunState) error {
	for _, policy := range a.policies {
		if err := policy.AfterModel(ctx, state); err != nil {
			return err
		}
	}
	return nil
}

func (a *Agent) afterTools(ctx context.Context, state *RunState) error {
	for _, policy := range a.policies {
		if err := policy.AfterTools(ctx, state); err != nil {
			return err
		}
	}
	return nil
}

func (a *Agent) executeToolCalls(ctx context.Context, state *RunState) {
	for _, toolCall := range state.Response.ToolCalls {
		originalToolCall := cloneToolCall(toolCall)
		toolCall, deniedResult, err := a.beforeToolCallHooks(ctx, state.Round, state.Request, toolCall)
		if err != nil {
			state.ToolCallResults = append(state.ToolCallResults, ToolCallResult{
				Call: originalToolCall,
				Err:  err,
			})
			continue
		}
		a.emit(ctx, Event{
			Type:         EventToolStart,
			Round:        state.Round,
			MessageCount: len(state.Session.Messages),
			ToolCount:    len(state.Request.Tools),
			ToolCall:     toolCall,
		})
		toolStart := time.Now()
		var toolResult ToolResult
		var toolErr error
		if deniedResult != nil {
			toolResult = normalizeToolResult(*deniedResult)
		} else {
			_, toolResult, toolErr = a.executeTool(ctx, toolCall)
		}
		hookedResult, err := a.afterToolCallHooks(ctx, state.Round, state.Request, toolCall, toolResult, toolErr)
		if err != nil {
			toolErr = err
			toolResult = ToolResult{Content: fmt.Sprintf("Error: %v", err), Error: true}
		} else {
			toolResult = hookedResult
		}
		toolContent := toolResult.Content
		toolCallResult := ToolCallResult{
			Call:     cloneToolCall(toolCall),
			Result:   toolContent,
			Error:    toolResult.Error,
			Metadata: cloneMetadata(toolResult.Metadata),
			Err:      toolErr,
			Duration: time.Since(toolStart),
		}
		state.Session.Messages = append(state.Session.Messages, ToolMessage(toolCall.ID, toolContent))
		a.emit(ctx, Event{
			Type:         EventToolResult,
			Round:        state.Round,
			MessageCount: len(state.Session.Messages),
			ToolCount:    len(state.Request.Tools),
			ToolCall:     toolCall,
			ToolResult:   toolContent,
			ToolError:    toolResult.Error,
			ToolMetadata: cloneMetadata(toolResult.Metadata),
			Duration:     time.Since(toolStart),
		})
		state.ToolCallResults = append(state.ToolCallResults, toolCallResult)
	}
}

func (a *Agent) complete(ctx context.Context, req Request, streamHandler StreamHandler) (Response, error) {
	if streamHandler == nil {
		return a.provider.Complete(ctx, req)
	}
	streamProvider, ok := a.provider.(StreamProvider)
	if !ok {
		resp, err := a.provider.Complete(ctx, req)
		if err != nil {
			return Response{}, err
		}
		if err := emitResponseStream(ctx, NewResponseStream(resp), streamHandler); err != nil {
			return Response{}, err
		}
		return resp, nil
	}
	stream, err := streamProvider.Stream(ctx, req)
	if err != nil {
		return Response{}, err
	}
	return ConsumeStream(ctx, stream, streamHandler)
}

func emitResponseStream(ctx context.Context, stream Stream, handler StreamHandler) error {
	_, err := ConsumeStream(ctx, stream, handler)
	return err
}

func (a *Agent) emit(ctx context.Context, event Event) {
	if a.eventHandler != nil {
		a.eventHandler(ctx, event)
	}
}

func (a *Agent) executeTool(ctx context.Context, toolCall ToolCall) (string, ToolResult, error) {
	if a.compaction != nil && a.compaction.Enabled && toolCall.Name == CompactionToolName {
		if _, ok := a.toolMap[CompactionToolName]; ok {
			err := errors.New("compact must be the only tool call in the assistant message")
			return toolCall.Name, ToolResult{Content: err.Error(), Error: true}, err
		}
	}
	tool, ok := a.toolMap[toolCall.Name]
	if !ok {
		err := fmt.Errorf("unknown tool: %s", toolCall.Name)
		return toolCall.Name, ToolResult{Content: fmt.Sprintf("Unknown tool: %s", toolCall.Name), Error: true}, err
	}
	handler := tool.StructuredHandler
	if handler == nil {
		handler = wrapToolHandler(tool.Handler)
	}
	if handler == nil {
		err := fmt.Errorf("tool %s has no handler", toolCall.Name)
		return toolCall.Name, ToolResult{Content: fmt.Sprintf("Error: %v", err), Error: true}, err
	}
	result, err := handler(ctx, toolCall.Arguments)
	if err != nil {
		if result.Content == "" {
			result.Content = fmt.Sprintf("Error: %v", err)
		}
		result.Error = true
		return toolCall.Name, normalizeToolResult(result), err
	}
	return toolCall.Name, normalizeToolResult(result), nil
}

func normalizeToolResult(result ToolResult) ToolResult {
	if result.Content == "" {
		result.Content = "(no output)"
	}
	result.Metadata = cloneMetadata(result.Metadata)
	return result
}

func (a *Agent) maybeAutoCompact(ctx context.Context, session *Session, roundNumber int) error {
	if a.compaction == nil || !a.compaction.Enabled {
		return nil
	}
	if a.compactionFailStreak >= maxConsecutiveCompactionFailures {
		return nil
	}
	cfg := *a.compaction
	microCompact(session.Messages, cfg.KeepRecentToolResults, cfg.MicroCompactMinChars, cfg.MicroCompactToolNames)
	req := Request{
		SystemPrompt: a.systemPrompt,
		Messages:     session.Messages,
		Tools:        a.tools,
	}
	if !inputTokensOverThreshold(req, cfg, session.lastCompleteInputTokens) {
		return nil
	}
	if _, err := saveCompactionTranscript(cfg.TranscriptDir, session.Messages); err != nil {
		return err
	}
	summary, err := summarizeCompaction(ctx, a.provider, cloneMessages(session.Messages))
	if err != nil {
		a.compactionFailStreak++
		if cfg.SkipOnSummarizeError {
			a.emit(ctx, Event{
				Type:         EventError,
				Round:        roundNumber,
				MessageCount: len(session.Messages),
				ToolCount:    len(a.tools),
				Err:          err,
			})
			return nil
		}
		return err
	}
	a.compactionFailStreak = 0
	session.Messages = []Message{UserMessage(compressedUserContent(summary))}
	session.lastCompleteInputTokens = 0
	return nil
}

func (a *Agent) runCompactionSummarize(ctx context.Context, session *Session, transcript []Message) (string, error) {
	if _, err := saveCompactionTranscript(a.compaction.TranscriptDir, transcript); err != nil {
		return "", err
	}
	summary, err := summarizeCompaction(ctx, a.provider, transcript)
	if err != nil {
		return "", err
	}
	session.Messages = []Message{UserMessage(compressedUserContent(summary))}
	session.lastCompleteInputTokens = 0
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
