package enno

import (
	"context"
	"testing"
)

type countingPolicy struct {
	beforeModel int
	afterModel  int
	afterTools  int
}

func (p *countingPolicy) BeforeModel(_ context.Context, state *RunState) error {
	p.beforeModel++
	if state.Round != p.beforeModel {
		panic("unexpected round")
	}
	return nil
}

func (p *countingPolicy) AfterModel(_ context.Context, state *RunState) error {
	p.afterModel++
	if state.Response.Content != "need tool" && state.Response.Content != "done" {
		panic("unexpected response")
	}
	return nil
}

func (p *countingPolicy) AfterTools(_ context.Context, state *RunState) error {
	p.afterTools++
	if len(state.ToolCallResults) != 1 {
		panic("expected one tool result")
	}
	return nil
}

func TestAgentPoliciesObserveRunState(t *testing.T) {
	policy := &countingPolicy{}
	provider := &sequenceProvider{responses: []Response{
		{
			Content: "need tool",
			ToolCalls: []ToolCall{{
				ID:   "call-1",
				Name: "echo",
				Arguments: []byte(`{
					"text": "hello"
				}`),
			}},
		},
		{Content: "done"},
	}}
	agent, err := NewAgent(Config{
		Provider: provider,
		Tools:    []Tool{runDetailedEchoTool()},
		Policies: []Policy{policy},
	})
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	if _, err := agent.RunDetailed(context.Background(), "start"); err != nil {
		t.Fatalf("RunDetailed: %v", err)
	}
	if policy.beforeModel != 2 || policy.afterModel != 1 || policy.afterTools != 1 {
		t.Fatalf("policy counts = before:%d afterModel:%d afterTools:%d", policy.beforeModel, policy.afterModel, policy.afterTools)
	}
}
