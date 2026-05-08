package enno

import (
	"context"
	"time"
)

type Policy interface {
	BeforeModel(context.Context, *RunState) error
	AfterModel(context.Context, *RunState) error
	AfterTools(context.Context, *RunState) error
}

type RunStartPolicy interface {
	OnRunStart()
}

type RunState struct {
	Round int

	Session *Session
	Request Request

	Response   Response
	ModelUsage Usage

	ToolCallResults   []ToolCallResult
	SkipToolExecution bool
}

type compactionPolicy struct {
	agent *Agent
}

func (p *compactionPolicy) OnRunStart() {
	p.agent.compactionFailStreak = 0
}

func (p *compactionPolicy) BeforeModel(ctx context.Context, state *RunState) error {
	return p.agent.maybeAutoCompact(ctx, state.Session, state.Round)
}

func (p *compactionPolicy) AfterModel(ctx context.Context, state *RunState) error {
	if p.agent.compaction == nil || !p.agent.compaction.Enabled {
		return nil
	}
	resp := state.Response
	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].Name != CompactionToolName {
		return nil
	}
	if _, ok := p.agent.toolMap[CompactionToolName]; !ok {
		return nil
	}

	assistantMsg := state.Session.Messages[len(state.Session.Messages)-1]
	state.Session.Messages = state.Session.Messages[:len(state.Session.Messages)-1]
	transcript := append(cloneMessages(state.Session.Messages), assistantMsg)
	toolCall := resp.ToolCalls[0]
	p.agent.emit(ctx, Event{
		Type:         EventToolStart,
		Round:        state.Round,
		MessageCount: len(state.Session.Messages),
		ToolCount:    len(state.Request.Tools),
		ToolCall:     toolCall,
	})
	toolStart := time.Now()
	toolResult, err := p.agent.runCompactionSummarize(ctx, state.Session, transcript)
	if err != nil {
		state.Session.Messages = append(state.Session.Messages, assistantMsg)
		p.agent.emit(ctx, Event{
			Type:         EventError,
			Round:        state.Round,
			MessageCount: len(state.Session.Messages),
			ToolCount:    len(state.Request.Tools),
			Err:          err,
		})
		return err
	}

	toolCallResult := ToolCallResult{
		Call:     cloneToolCall(toolCall),
		Result:   toolResult,
		Duration: time.Since(toolStart),
	}
	p.agent.emit(ctx, Event{
		Type:         EventToolResult,
		Round:        state.Round,
		MessageCount: len(state.Session.Messages),
		ToolCount:    len(state.Request.Tools),
		ToolCall:     toolCall,
		ToolResult:   toolResult,
		Duration:     time.Since(toolStart),
	})
	state.ToolCallResults = append(state.ToolCallResults, toolCallResult)
	state.SkipToolExecution = true
	return nil
}

func (p *compactionPolicy) AfterTools(context.Context, *RunState) error {
	return nil
}

type taskReminderPolicy struct {
	roundsSincePlan int
}

func (p *taskReminderPolicy) OnRunStart() {
	p.roundsSincePlan = 0
}

func (p *taskReminderPolicy) BeforeModel(context.Context, *RunState) error {
	return nil
}

func (p *taskReminderPolicy) AfterModel(context.Context, *RunState) error {
	return nil
}

func (p *taskReminderPolicy) AfterTools(_ context.Context, state *RunState) error {
	usedPlanTool := false
	for _, toolCall := range state.ToolCallResults {
		if isTaskGraphToolName(toolCall.Call.Name) {
			usedPlanTool = true
			break
		}
	}
	if usedPlanTool {
		p.roundsSincePlan = 0
	} else {
		p.roundsSincePlan++
	}
	if p.roundsSincePlan >= 3 {
		state.Session.Messages = append(state.Session.Messages, UserMessage("<reminder>Update your task plan.</reminder>"))
	}
	return nil
}
