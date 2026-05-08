package enno

import (
	"context"
	"errors"
	"testing"
)

type requestCaptureProvider struct {
	requests  []Request
	response  Response
	lastError error
}

func (p *requestCaptureProvider) Complete(_ context.Context, req Request) (Response, error) {
	p.requests = append(p.requests, cloneRequest(req))
	if p.lastError != nil {
		return Response{}, p.lastError
	}
	return p.response, nil
}

type providerHook struct {
	NoopHook
}

func (providerHook) BeforeProviderCall(_ context.Context, state BeforeProviderCallState) (BeforeProviderCallResult, error) {
	req := state.Request
	req.SystemPrompt = "hooked"
	return BeforeProviderCallResult{Request: &req}, nil
}

func (providerHook) AfterProviderCall(_ context.Context, state AfterProviderCallState) (AfterProviderCallResult, error) {
	resp := state.Response
	resp.Content = "replaced"
	return AfterProviderCallResult{Response: &resp}, nil
}

func TestProviderHooksCanReplaceRequestAndResponse(t *testing.T) {
	provider := &requestCaptureProvider{response: Response{Content: "original"}}
	agent, err := NewAgent(Config{
		Provider: provider,
		Hooks:    []Hook{providerHook{}},
	})
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	result, err := agent.Run(context.Background(), &Session{}, "start")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if provider.requests[0].SystemPrompt != "hooked" {
		t.Fatalf("provider request = %#v", provider.requests[0])
	}
	if result.Content != "replaced" {
		t.Fatalf("content = %q, want replaced", result.Content)
	}
}

type denyToolHook struct {
	NoopHook
}

func (denyToolHook) BeforeToolCall(context.Context, BeforeToolCallState) (BeforeToolCallResult, error) {
	return BeforeToolCallResult{
		Deny:        true,
		DenyMessage: "denied",
	}, nil
}

func TestBeforeToolHookCanDenyTool(t *testing.T) {
	provider := &sequenceProvider{responses: []Response{
		{
			ToolCalls: []ToolCall{{
				ID:        "call-1",
				Name:      "echo",
				Arguments: []byte(`{"text":"hello"}`),
			}},
		},
		{Content: "done"},
	}}
	called := false
	agent, err := NewAgent(Config{
		Provider: provider,
		Tools: []Tool{NewTypedTool("echo", "Echo.", map[string]any{
			"text": map[string]any{"type": "string"},
		}, []string{"text"}, func(context.Context, struct {
			Text string `json:"text"`
		}) (string, error) {
			called = true
			return "should not run", nil
		})},
		Hooks: []Hook{denyToolHook{}},
	})
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	result, err := agent.Run(context.Background(), &Session{}, "start")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if called {
		t.Fatal("tool handler should not be called when hook denies")
	}
	toolResult := result.Rounds[0].ToolCalls[0]
	if !toolResult.Error || toolResult.Result != "denied" {
		t.Fatalf("tool result = %#v", toolResult)
	}
}

type abortHook struct {
	NoopHook
	err error
}

func (h abortHook) BeforeProviderCall(context.Context, BeforeProviderCallState) (BeforeProviderCallResult, error) {
	return BeforeProviderCallResult{Abort: h.err}, nil
}

func TestHookCanAbortRun(t *testing.T) {
	abortErr := errors.New("blocked")
	agent, err := NewAgent(Config{
		Provider: staticProvider{resp: Response{Content: "unused"}},
		Hooks:    []Hook{abortHook{err: abortErr}},
	})
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	result, err := agent.Run(context.Background(), &Session{}, "start")
	if !errors.Is(err, abortErr) {
		t.Fatalf("Run error = %v, want %v", err, abortErr)
	}
	if result.StopReason != StopReasonError {
		t.Fatalf("StopReason = %q, want %q", result.StopReason, StopReasonError)
	}
}
