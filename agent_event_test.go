package enno

import (
	"context"
	"encoding/json"
	"testing"
)

type eventProvider struct {
	calls int
}

func (p *eventProvider) Complete(_ context.Context, _ Request) (Response, error) {
	p.calls++
	if p.calls == 1 {
		return Response{
			Content: "need tool",
			ToolCalls: []ToolCall{{
				ID:        "call-1",
				Name:      "echo",
				Arguments: json.RawMessage(`{"text":"hello"}`),
			}},
			Usage: Usage{InputTokens: 10, OutputTokens: 2, TotalTokens: 12},
		}, nil
	}
	return Response{
		Content: "done",
		Usage:   Usage{InputTokens: 15, OutputTokens: 3, TotalTokens: 18},
	}, nil
}

func TestAgentEmitsModelAndToolEvents(t *testing.T) {
	var events []Event
	agent, err := NewAgent(Config{
		Provider: &eventProvider{},
		Tools: []Tool{NewTypedTool("echo", "Echo text.", map[string]any{
			"text": map[string]any{"type": "string"},
		}, []string{"text"}, func(_ context.Context, args struct {
			Text string `json:"text"`
		}) (string, error) {
			return args.Text, nil
		})},
		EventHandler: func(_ context.Context, event Event) {
			events = append(events, event)
		},
	})
	if err != nil {
		t.Fatalf("new agent: %v", err)
	}

	answer, err := agent.Run(context.Background(), "start")
	if err != nil {
		t.Fatalf("run agent: %v", err)
	}
	if answer != "done" {
		t.Fatalf("expected final answer, got %q", answer)
	}

	want := []EventType{
		EventModelStart,
		EventModelResponse,
		EventToolStart,
		EventToolResult,
		EventRoundComplete,
		EventModelStart,
		EventModelResponse,
		EventRoundComplete,
	}
	if len(events) != len(want) {
		t.Fatalf("expected %d events, got %d: %#v", len(want), len(events), events)
	}
	for i, eventType := range want {
		if events[i].Type != eventType {
			t.Fatalf("event %d: expected %s, got %s", i, eventType, events[i].Type)
		}
	}
	if events[1].Usage.TotalTokens != 12 {
		t.Fatalf("expected model usage to be emitted, got %#v", events[1].Usage)
	}
	if events[2].ToolCall.Name != "echo" {
		t.Fatalf("expected tool_start for echo, got %#v", events[2].ToolCall)
	}
	if events[3].ToolResult != "hello" {
		t.Fatalf("expected tool result, got %q", events[3].ToolResult)
	}
}
