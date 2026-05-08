package enno

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type recordingProvider struct {
	responses []Response
	requests  []Request
}

func (p *recordingProvider) Complete(_ context.Context, req Request) (Response, error) {
	p.requests = append(p.requests, Request{
		SystemPrompt: req.SystemPrompt,
		Messages:     cloneMessages(req.Messages),
		Tools:        append([]Tool(nil), req.Tools...),
	})
	if len(p.responses) == 0 {
		return Response{Content: "ok"}, nil
	}
	resp := p.responses[0]
	p.responses = p.responses[1:]
	return resp, nil
}

func TestSessionAppendCloneAndReset(t *testing.T) {
	rawArgs := json.RawMessage(`{"name":"alpha"}`)
	session := Session{}
	session.Append(AssistantMessage("hello", []ToolCall{{
		ID:        "call-1",
		Name:      "tool",
		Arguments: rawArgs,
	}}))

	rawArgs[9] = 'X'
	if string(session.Messages[0].ToolCalls[0].Arguments) != `{"name":"alpha"}` {
		t.Fatalf("Append should clone tool call arguments, got %s", session.Messages[0].ToolCalls[0].Arguments)
	}

	clone := session.Clone()
	clone.Messages[0].Content = "changed"
	clone.Messages[0].ToolCalls[0].Arguments[9] = 'Y'
	if session.Messages[0].Content != "hello" {
		t.Fatalf("Clone should not share messages, got %q", session.Messages[0].Content)
	}
	if string(session.Messages[0].ToolCalls[0].Arguments) != `{"name":"alpha"}` {
		t.Fatalf("Clone should not share tool call arguments, got %s", session.Messages[0].ToolCalls[0].Arguments)
	}

	session.Reset()
	if session.Messages != nil {
		t.Fatalf("Reset should clear messages, got %#v", session.Messages)
	}
}

func TestAgentRunSessionUsesExternalSession(t *testing.T) {
	provider := &recordingProvider{responses: []Response{{Content: "answer"}}}
	agent, err := NewAgent(Config{Provider: provider})
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}
	session := Session{}
	session.Append(UserMessage("prior"))

	result, err := agent.RunSession(context.Background(), &session, "next")
	if err != nil {
		t.Fatalf("RunSession: %v", err)
	}

	if result.Content != "answer" {
		t.Fatalf("Content = %q, want answer", result.Content)
	}
	if len(provider.requests) != 1 {
		t.Fatalf("provider calls = %d, want 1", len(provider.requests))
	}
	reqMessages := provider.requests[0].Messages
	if len(reqMessages) != 2 || reqMessages[0].Content != "prior" || reqMessages[1].Content != "next" {
		t.Fatalf("request messages = %#v", reqMessages)
	}
	if len(session.Messages) != 3 {
		t.Fatalf("session messages = %#v, want prior + next + assistant", session.Messages)
	}
	if len(agent.Messages()) != 0 {
		t.Fatalf("RunSession should not mutate agent default session, got %#v", agent.Messages())
	}
}

func TestAgentRunSessionCanContinuePersistedConversation(t *testing.T) {
	provider := &recordingProvider{responses: []Response{
		{Content: "first"},
		{Content: "second"},
	}}
	agent, err := NewAgent(Config{Provider: provider})
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}
	session := Session{}

	if _, err := agent.RunSession(context.Background(), &session, "one"); err != nil {
		t.Fatalf("first RunSession: %v", err)
	}
	if _, err := agent.RunSession(context.Background(), &session, "two"); err != nil {
		t.Fatalf("second RunSession: %v", err)
	}

	if len(provider.requests) != 2 {
		t.Fatalf("provider calls = %d, want 2", len(provider.requests))
	}
	secondRequest := provider.requests[1].Messages
	if len(secondRequest) != 3 {
		t.Fatalf("second request messages = %#v", secondRequest)
	}
	if secondRequest[0].Content != "one" || secondRequest[1].Content != "first" || secondRequest[2].Content != "two" {
		t.Fatalf("second request did not include prior session state: %#v", secondRequest)
	}
}

func TestAgentRunSessionNilSession(t *testing.T) {
	agent, err := NewAgent(Config{Provider: staticProvider{resp: Response{Content: "unused"}}})
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	result, err := agent.RunSession(context.Background(), nil, "input")
	if !errors.Is(err, ErrNilSession) {
		t.Fatalf("RunSession error = %v, want %v", err, ErrNilSession)
	}
	if result.StopReason != StopReasonError {
		t.Fatalf("StopReason = %q, want %q", result.StopReason, StopReasonError)
	}
}
