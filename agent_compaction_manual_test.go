package enno_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dean2021/enno"
	compacttool "github.com/dean2021/enno/builtintools/compact"
)

type manualCompactProvider struct {
	calls                int
	summarizeWrongPrompt bool
}

func (p *manualCompactProvider) Complete(_ context.Context, req enno.Request) (enno.Response, error) {
	p.calls++
	switch p.calls {
	case 1:
		return enno.Response{
			Content: "",
			ToolCalls: []enno.ToolCall{{
				ID:        "c1",
				Name:      enno.CompactionToolName,
				Arguments: json.RawMessage(`{}`),
			}},
		}, nil
	case 2:
		// Summarize call — system prompt is package-private; match via messages shape.
		if len(req.Messages) != 1 || req.Messages[0].Role != enno.RoleUser {
			p.summarizeWrongPrompt = true
		}
		return enno.Response{Content: "rolled-up summary"}, nil
	default:
		return enno.Response{Content: "final reply"}, nil
	}
}

func TestManualCompactionSingleCompactToolCall(t *testing.T) {
	tool := compacttool.New()
	if tool.Name != enno.CompactionToolName {
		t.Fatalf("tool name: %q", tool.Name)
	}
	p := &manualCompactProvider{}
	agent, err := enno.NewAgent(enno.Config{
		Provider: p,
		Tools:    []enno.Tool{tool},
		Compaction: &enno.CompactionConfig{
			Enabled:       true,
			TranscriptDir: t.TempDir(),
		},
	})
	if err != nil {
		t.Fatalf("new agent: %v", err)
	}
	session := enno.Session{}
	result, err := agent.Run(context.Background(), &session, "hello")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Content != "final reply" {
		t.Fatalf("answer %q", result.Content)
	}
	if p.calls != 3 {
		t.Fatalf("expected chat + summarize + chat Complete calls, got %d", p.calls)
	}
	if p.summarizeWrongPrompt {
		t.Fatal("summarize Complete had unexpected request shape")
	}
	msgs := session.Messages
	if len(msgs) != 2 {
		t.Fatalf("messages: %d", len(msgs))
	}
	if msgs[0].Role != enno.RoleUser || msgs[0].Content[:12] != "[Compressed]" {
		t.Fatalf("first msg: %#v", msgs[0])
	}
	if msgs[1].Role != enno.RoleAssistant || msgs[1].Content != "final reply" {
		t.Fatalf("second msg: %#v", msgs[1])
	}
}
