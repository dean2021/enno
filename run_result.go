package enno

import (
	"encoding/json"
	"time"
)

type StopReason string

const (
	StopReasonEndTurn       StopReason = "end_turn"
	StopReasonMaxToolRounds StopReason = "max_tool_rounds"
	StopReasonError         StopReason = "error"
	StopReasonCanceled      StopReason = "canceled"
)

type RunResult struct {
	Content    string
	Messages   []Message
	Usage      Usage
	Rounds     []RoundResult
	StopReason StopReason
	Duration   time.Duration
}

type RoundResult struct {
	Round      int
	ModelUsage Usage
	Assistant  Message
	ToolCalls  []ToolCallResult
	Duration   time.Duration
}

type ToolCallResult struct {
	Call     ToolCall
	Result   string
	Error    bool
	Metadata map[string]any
	Err      error
	Duration time.Duration
}

func addUsage(total *Usage, usage Usage) {
	total.InputTokens += usage.InputTokens
	total.OutputTokens += usage.OutputTokens
	total.TotalTokens += usage.TotalTokens
	total.Estimated = total.Estimated || usage.Estimated
}

func cloneMessages(messages []Message) []Message {
	if len(messages) == 0 {
		return nil
	}
	cloned := make([]Message, len(messages))
	for i, message := range messages {
		cloned[i] = cloneMessage(message)
	}
	return cloned
}

func cloneMessage(message Message) Message {
	message.ToolCalls = cloneToolCalls(message.ToolCalls)
	return message
}

func cloneToolCalls(toolCalls []ToolCall) []ToolCall {
	if len(toolCalls) == 0 {
		return nil
	}
	cloned := make([]ToolCall, len(toolCalls))
	for i, toolCall := range toolCalls {
		cloned[i] = cloneToolCall(toolCall)
	}
	return cloned
}

func cloneToolCall(toolCall ToolCall) ToolCall {
	if len(toolCall.Arguments) > 0 {
		toolCall.Arguments = append(json.RawMessage(nil), toolCall.Arguments...)
	}
	return toolCall
}

func cloneMetadata(metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(metadata))
	for key, value := range metadata {
		cloned[key] = value
	}
	return cloned
}
