package enno

import (
	"context"
	"encoding/json"
	"time"
)

type Usage struct {
	InputTokens  int64
	OutputTokens int64
	TotalTokens  int64
	Estimated    bool
}

type EventType string

const (
	EventModelStart    EventType = "model_start"
	EventModelResponse EventType = "model_response"
	EventToolStart     EventType = "tool_start"
	EventToolResult    EventType = "tool_result"
	EventRoundComplete EventType = "round_complete"
	EventError         EventType = "error"
)

type EventHandler func(context.Context, Event)

type Event struct {
	Type         EventType
	Round        int
	MessageCount int
	ToolCount    int
	Content      string
	ToolCall     ToolCall
	ToolResult   string
	Usage        Usage
	Duration     time.Duration
	Err          error
}

func EstimateUsage(req Request) Usage {
	chars := len(req.SystemPrompt)
	for _, message := range req.Messages {
		chars += len(message.Content)
		for _, toolCall := range message.ToolCalls {
			chars += len(toolCall.Name) + len(toolCall.Arguments)
		}
	}
	for _, tool := range req.Tools {
		chars += len(tool.Name) + len(tool.Description)
		if len(tool.Properties) > 0 {
			if bytes, err := json.Marshal(tool.Properties); err == nil {
				chars += len(bytes)
			}
		}
	}
	return Usage{
		InputTokens: int64((chars + 3) / 4),
		Estimated:   true,
	}
}

func (u Usage) withTotal() Usage {
	if u.TotalTokens <= 0 {
		u.TotalTokens = u.InputTokens + u.OutputTokens
	}
	return u
}
