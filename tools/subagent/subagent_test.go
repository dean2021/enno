package subagent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/dean2021/enno"
)

func TestNewRejectsRecursiveChildTool(t *testing.T) {
	_, err := New(Config{
		Provider:   &stubProvider{},
		ChildTools: []enno.Tool{fakeTool("echo"), fakeTool(DefaultToolName)},
	})
	if err == nil || !strings.Contains(err.Error(), "recursive") {
		t.Fatalf("expected recursive dispatch error, got %v", err)
	}
}

func TestChildAgentDoesNotReceiveTaskTool(t *testing.T) {
	rec := &recordingProvider{steps: []recProviderStep{
		{
			resp: enno.Response{
				Content: "call echo",
				ToolCalls: []enno.ToolCall{{
					ID: "1", Name: "echo", Arguments: json.RawMessage(`{"text":"x"}`),
				}},
			},
		},
		{resp: enno.Response{Content: "done"}},
	}}
	childOnly := []enno.Tool{fakeTool("echo")}
	task, err := New(Config{
		Provider:   rec,
		ChildTools: childOnly,
		ToolName:   "delegate",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	out, err := task.Handler(context.Background(), json.RawMessage(`{"prompt":"run echo"}`))
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if out != "done" {
		t.Fatalf("expected child final text %q, got %q", "done", out)
	}
	if len(rec.requests) < 2 {
		t.Fatalf("expected at least 2 Complete calls, got %d", len(rec.requests))
	}
	first := rec.requests[0]
	var names []string
	for _, tool := range first.Tools {
		names = append(names, tool.Name)
	}
	for _, n := range names {
		if n == "delegate" || n == DefaultToolName {
			t.Fatalf("child request must not include subagent tool, got tools %#v", names)
		}
	}
	if len(names) != 1 || names[0] != "echo" {
		t.Fatalf("expected child tools [echo], got %#v", names)
	}
}

func TestConfiguredMaxResultChars(t *testing.T) {
	p := &recordingProvider{steps: []recProviderStep{
		{resp: enno.Response{Content: strings.Repeat("a", 100)}},
	}}
	task, err := New(Config{
		Provider:       p,
		ChildTools:     []enno.Tool{fakeTool("echo")},
		MaxResultChars: 50,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got, err := task.Handler(context.Background(), json.RawMessage(`{"prompt":"x"}`))
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if len([]rune(got)) > 50 {
		t.Fatalf("expected len <= 50, got %d", len([]rune(got)))
	}
	if !strings.HasSuffix(got, "[truncated]") {
		t.Fatalf("expected suffix, got %q", got)
	}
}

func TestEmptyFinalMessageUsesPlaceholder(t *testing.T) {
	p := &recordingProvider{steps: []recProviderStep{
		{resp: enno.Response{Content: "   "}},
	}}
	task, err := New(Config{Provider: p, ChildTools: []enno.Tool{fakeTool("echo")}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	out, err := task.Handler(context.Background(), json.RawMessage(`{"prompt":"x"}`))
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if out != "(no summary)" {
		t.Fatalf("expected placeholder, got %q", out)
	}
}

type recProviderStep struct {
	resp enno.Response
	err  error
}

type recordingProvider struct {
	steps    []recProviderStep
	i        int
	requests []enno.Request
}

func (p *recordingProvider) Complete(_ context.Context, req enno.Request) (enno.Response, error) {
	p.requests = append(p.requests, req)
	if p.i >= len(p.steps) {
		return enno.Response{Content: "fallback"}, nil
	}
	step := p.steps[p.i]
	p.i++
	return step.resp, step.err
}

type stubProvider struct{}

func (p *stubProvider) Complete(context.Context, enno.Request) (enno.Response, error) {
	return enno.Response{Content: "ok"}, nil
}

func fakeTool(name string) enno.Tool {
	return enno.NewTool(name, "test", map[string]any{}, nil, func(context.Context, json.RawMessage) (string, error) {
		return "", nil
	})
}
