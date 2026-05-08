package enno

import (
	"context"
	"io"
	"testing"
)

type fakeStreamProvider struct {
	events []StreamEvent
}

func (p *fakeStreamProvider) Complete(context.Context, Request) (Response, error) {
	return Response{Content: "non-stream"}, nil
}

func (p *fakeStreamProvider) Stream(context.Context, Request) (Stream, error) {
	return &fakeStream{events: append([]StreamEvent(nil), p.events...)}, nil
}

type fakeStream struct {
	events []StreamEvent
	index  int
}

func (s *fakeStream) Next(context.Context) (StreamEvent, error) {
	if s.index >= len(s.events) {
		return StreamEvent{}, io.EOF
	}
	event := s.events[s.index]
	s.index++
	return event, nil
}

func (s *fakeStream) Close() error {
	return nil
}

func TestAgentRunStreamUsesStreamProvider(t *testing.T) {
	provider := &fakeStreamProvider{events: []StreamEvent{
		{Type: StreamEventTextDelta, Text: "hel"},
		{Type: StreamEventTextDelta, Text: "lo"},
		{Type: StreamEventUsage, Usage: Usage{InputTokens: 1, OutputTokens: 2, TotalTokens: 3}},
		{Type: StreamEventFinalResponse, Response: Response{
			Content: "hello",
			Usage:   Usage{InputTokens: 1, OutputTokens: 2, TotalTokens: 3},
		}},
	}}
	agent, err := NewAgent(Config{Provider: provider})
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	var events []StreamEvent
	result, err := agent.RunStream(context.Background(), "start", func(_ context.Context, event StreamEvent) {
		events = append(events, event)
	})
	if err != nil {
		t.Fatalf("RunStream: %v", err)
	}
	if result.Content != "hello" {
		t.Fatalf("Content = %q, want hello", result.Content)
	}
	if result.Usage.TotalTokens != 3 {
		t.Fatalf("Usage = %#v", result.Usage)
	}
	if len(events) != 4 {
		t.Fatalf("events = %#v", events)
	}
}

func TestAgentRunStreamFallsBackToComplete(t *testing.T) {
	agent, err := NewAgent(Config{
		Provider: staticProvider{resp: Response{Content: "fallback"}},
	})
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	var events []StreamEvent
	result, err := agent.RunStream(context.Background(), "start", func(_ context.Context, event StreamEvent) {
		events = append(events, event)
	})
	if err != nil {
		t.Fatalf("RunStream: %v", err)
	}
	if result.Content != "fallback" {
		t.Fatalf("Content = %q, want fallback", result.Content)
	}
	if len(events) != 2 {
		t.Fatalf("events = %#v", events)
	}
	if events[0].Type != StreamEventTextDelta || events[1].Type != StreamEventFinalResponse {
		t.Fatalf("events = %#v", events)
	}
}
