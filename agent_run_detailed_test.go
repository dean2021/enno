package enno

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type staticProvider struct {
	resp Response
	err  error
}

func (p staticProvider) Complete(_ context.Context, _ Request) (Response, error) {
	return p.resp, p.err
}

type sequenceProvider struct {
	responses []Response
	calls     int
}

func (p *sequenceProvider) Complete(_ context.Context, _ Request) (Response, error) {
	if p.calls >= len(p.responses) {
		return Response{Content: "extra"}, nil
	}
	resp := p.responses[p.calls]
	p.calls++
	return resp, nil
}

func runDetailedEchoTool() Tool {
	return NewTypedTool("echo", "Echo.", map[string]any{
		"text": map[string]any{"type": "string"},
	}, []string{"text"}, func(_ context.Context, args struct {
		Text string `json:"text"`
	}) (string, error) {
		return args.Text, nil
	})
}

func TestAgentRunTextResponse(t *testing.T) {
	agent, err := NewAgent(Config{
		Provider: staticProvider{resp: Response{
			Content: "done",
			Usage:   Usage{InputTokens: 7, OutputTokens: 3},
		}},
	})
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	result, err := agent.Run(context.Background(), &Session{}, "start")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if result.Content != "done" {
		t.Fatalf("Content = %q, want done", result.Content)
	}
	if result.StopReason != StopReasonEndTurn {
		t.Fatalf("StopReason = %q, want %q", result.StopReason, StopReasonEndTurn)
	}
	if result.Usage.TotalTokens != 10 {
		t.Fatalf("Usage.TotalTokens = %d, want 10", result.Usage.TotalTokens)
	}
	if len(result.Rounds) != 1 {
		t.Fatalf("len(Rounds) = %d, want 1", len(result.Rounds))
	}
	if result.Rounds[0].Assistant.Content != "done" {
		t.Fatalf("round assistant = %#v", result.Rounds[0].Assistant)
	}
	if len(result.Messages) != 2 {
		t.Fatalf("len(Messages) = %d, want 2", len(result.Messages))
	}
	if result.Messages[0].Role != RoleUser || result.Messages[1].Role != RoleAssistant {
		t.Fatalf("Messages = %#v", result.Messages)
	}
}

func TestAgentRunReturnsDetailedResult(t *testing.T) {
	agent, err := NewAgent(Config{
		Provider: staticProvider{resp: Response{Content: "plain"}},
	})
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	result, err := agent.Run(context.Background(), &Session{}, "start")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Content != "plain" {
		t.Fatalf("answer = %q, want plain", result.Content)
	}
}

func TestAgentRunToolCallResponse(t *testing.T) {
	provider := &sequenceProvider{responses: []Response{
		{
			Content: "need tool",
			ToolCalls: []ToolCall{{
				ID:        "call-1",
				Name:      "echo",
				Arguments: json.RawMessage(`{"text":"hello"}`),
			}},
			Usage: Usage{InputTokens: 10, OutputTokens: 2, TotalTokens: 12},
		},
		{
			Content: "done",
			Usage:   Usage{InputTokens: 15, OutputTokens: 3, TotalTokens: 18},
		},
	}}
	agent, err := NewAgent(Config{
		Provider: provider,
		Tools:    []Tool{runDetailedEchoTool()},
	})
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	result, err := agent.Run(context.Background(), &Session{}, "start")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if result.Content != "done" {
		t.Fatalf("Content = %q, want done", result.Content)
	}
	if len(result.Rounds) != 2 {
		t.Fatalf("len(Rounds) = %d, want 2", len(result.Rounds))
	}
	if len(result.Rounds[0].ToolCalls) != 1 {
		t.Fatalf("len(Rounds[0].ToolCalls) = %d, want 1", len(result.Rounds[0].ToolCalls))
	}
	toolResult := result.Rounds[0].ToolCalls[0]
	if toolResult.Call.Name != "echo" || toolResult.Result != "hello" || toolResult.Err != nil {
		t.Fatalf("tool result = %#v", toolResult)
	}
	if result.Usage.TotalTokens != 30 {
		t.Fatalf("Usage.TotalTokens = %d, want 30", result.Usage.TotalTokens)
	}
	if len(result.Messages) != 4 {
		t.Fatalf("len(Messages) = %d, want 4", len(result.Messages))
	}
	if result.Messages[2].Role != RoleTool || result.Messages[2].Content != "hello" {
		t.Fatalf("tool message = %#v", result.Messages[2])
	}
}

func TestAgentRunMaxToolRounds(t *testing.T) {
	provider := &sequenceProvider{responses: []Response{
		{
			ToolCalls: []ToolCall{{
				ID:        "call-1",
				Name:      "echo",
				Arguments: json.RawMessage(`{"text":"again"}`),
			}},
		},
	}}
	agent, err := NewAgent(Config{
		Provider:      provider,
		Tools:         []Tool{runDetailedEchoTool()},
		MaxToolRounds: 1,
	})
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	result, err := agent.Run(context.Background(), &Session{}, "start")
	if err == nil {
		t.Fatal("Run: expected error")
	}
	if !strings.Contains(err.Error(), "agent exceeded max tool rounds: 1") {
		t.Fatalf("error = %v", err)
	}
	if result.StopReason != StopReasonMaxToolRounds {
		t.Fatalf("StopReason = %q, want %q", result.StopReason, StopReasonMaxToolRounds)
	}
	if len(result.Rounds) != 1 {
		t.Fatalf("len(Rounds) = %d, want 1", len(result.Rounds))
	}
	if result.Rounds[0].ToolCalls[0].Result != "again" {
		t.Fatalf("tool result = %#v", result.Rounds[0].ToolCalls[0])
	}
	if len(result.Messages) != 3 {
		t.Fatalf("len(Messages) = %d, want 3", len(result.Messages))
	}

	runAgent, err := NewAgent(Config{
		Provider: &sequenceProvider{responses: []Response{
			{
				ToolCalls: []ToolCall{{
					ID:        "call-1",
					Name:      "echo",
					Arguments: json.RawMessage(`{"text":"again"}`),
				}},
			},
		}},
		Tools:         []Tool{runDetailedEchoTool()},
		MaxToolRounds: 1,
	})
	if err != nil {
		t.Fatalf("NewAgent for Run: %v", err)
	}
	result, err = runAgent.Run(context.Background(), &Session{}, "start")
	if err == nil {
		t.Fatal("Run: expected error")
	}
	if result.Content != "" {
		t.Fatalf("Run answer = %q, want empty string on error", result.Content)
	}
}

func TestAgentRunProviderError(t *testing.T) {
	providerErr := errors.New("provider failed")
	agent, err := NewAgent(Config{
		Provider: staticProvider{err: providerErr},
	})
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	result, err := agent.Run(context.Background(), &Session{}, "start")
	if !errors.Is(err, providerErr) {
		t.Fatalf("Run error = %v, want %v", err, providerErr)
	}
	if result.StopReason != StopReasonError {
		t.Fatalf("StopReason = %q, want %q", result.StopReason, StopReasonError)
	}
	if len(result.Rounds) != 0 {
		t.Fatalf("len(Rounds) = %d, want 0", len(result.Rounds))
	}
	if len(result.Messages) != 1 || result.Messages[0].Content != "start" {
		t.Fatalf("Messages = %#v", result.Messages)
	}
}

func TestAgentRunToolErrorIsCaptured(t *testing.T) {
	provider := &sequenceProvider{responses: []Response{
		{
			ToolCalls: []ToolCall{{
				ID:        "call-1",
				Name:      "fail",
				Arguments: json.RawMessage(`{}`),
			}},
		},
		{Content: "done"},
	}}
	toolErr := errors.New("tool failed")
	agent, err := NewAgent(Config{
		Provider: provider,
		Tools: []Tool{NewTool("fail", "Fail.", nil, nil, func(_ context.Context, _ json.RawMessage) (string, error) {
			return "", toolErr
		})},
	})
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	result, err := agent.Run(context.Background(), &Session{}, "start")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	toolResult := result.Rounds[0].ToolCalls[0]
	if !errors.Is(toolResult.Err, toolErr) {
		t.Fatalf("tool error = %v, want %v", toolResult.Err, toolErr)
	}
	if !toolResult.Error {
		t.Fatal("tool result should be marked as error")
	}
	if toolResult.Result != "Error: tool failed" {
		t.Fatalf("tool result = %q, want model-visible error", toolResult.Result)
	}
}

func TestAgentRunStructuredToolResult(t *testing.T) {
	provider := &sequenceProvider{responses: []Response{
		{
			ToolCalls: []ToolCall{{
				ID:        "call-1",
				Name:      "lookup",
				Arguments: json.RawMessage(`{"id":"42"}`),
			}},
		},
		{Content: "done"},
	}}
	var events []Event
	agent, err := NewAgent(Config{
		Provider: provider,
		Tools: []Tool{NewStructuredTool("lookup", "Lookup.", map[string]any{
			"id": map[string]any{"type": "string"},
		}, []string{"id"}, func(_ context.Context, raw json.RawMessage) (ToolResult, error) {
			return ToolResult{
				Content: "visible content",
				Error:   true,
				Metadata: map[string]any{
					"source": "cache",
				},
			}, nil
		})},
		EventHandler: func(_ context.Context, event Event) {
			events = append(events, event)
		},
	})
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	result, err := agent.Run(context.Background(), &Session{}, "start")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	toolResult := result.Rounds[0].ToolCalls[0]
	if toolResult.Result != "visible content" {
		t.Fatalf("tool result = %q, want visible content", toolResult.Result)
	}
	if !toolResult.Error {
		t.Fatal("tool result should preserve structured error flag")
	}
	if toolResult.Metadata["source"] != "cache" {
		t.Fatalf("tool metadata = %#v", toolResult.Metadata)
	}
	if result.Messages[2].Content != "visible content" {
		t.Fatalf("model-visible tool message = %#v", result.Messages[2])
	}

	var toolEvent Event
	for _, event := range events {
		if event.Type == EventToolResult {
			toolEvent = event
			break
		}
	}
	if toolEvent.Type != EventToolResult {
		t.Fatal("expected tool result event")
	}
	if !toolEvent.ToolError || toolEvent.ToolMetadata["source"] != "cache" {
		t.Fatalf("tool event = %#v", toolEvent)
	}
}
