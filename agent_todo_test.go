package enno

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// streakEchoProvider returns an echo tool call for the first n Complete invocations, then plain text.
type streakEchoProvider struct {
	calls        int
	streakRounds int
}

func (p *streakEchoProvider) Complete(_ context.Context, _ Request) (Response, error) {
	p.calls++
	if p.calls <= p.streakRounds {
		return Response{
			ToolCalls: []ToolCall{{
				ID:        "call-echo",
				Name:      "echo",
				Arguments: json.RawMessage(`{"text":"x"}`),
			}},
		}, nil
	}
	return Response{Content: "done"}, nil
}

func echoTool() Tool {
	return NewTypedTool("echo", "Echo.", map[string]any{
		"text": map[string]any{"type": "string"},
	}, []string{"text"}, func(_ context.Context, args struct {
		Text string `json:"text"`
	}) (string, error) {
		return args.Text, nil
	})
}

func TestAgent_NoTodoTool_DoesNotInjectReminder(t *testing.T) {
	p := &streakEchoProvider{streakRounds: 4}
	agent, err := NewAgent(Config{
		Provider: p,
		Tools:    []Tool{echoTool()},
	})
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}
	if _, err := agent.Run(context.Background(), "start"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	raw := messagesString(agent.Messages())
	if strings.Contains(raw, "<reminder>Update your todos.</reminder>") {
		t.Fatalf("did not expect todo reminder without todo tool, messages:\n%s", raw)
	}
}

// stubTodoTool registers the name "todo" without importing tools/todo (enno must not depend on tools/*).
func stubTodoTool() Tool {
	return NewTool("todo", "stub todo for tests", map[string]any{
		"items": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
			},
		},
	}, []string{"items"}, func(_ context.Context, _ json.RawMessage) (string, error) {
		return "ok", nil
	})
}

func TestAgent_WithTodoTool_InjectsReminderAfterThreeToolRoundsWithoutTodo(t *testing.T) {
	p := &streakEchoProvider{streakRounds: 4}
	agent, err := NewAgent(Config{
		Provider: p,
		Tools:    []Tool{echoTool(), stubTodoTool()},
	})
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}
	if _, err := agent.Run(context.Background(), "start"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	raw := messagesString(agent.Messages())
	if !strings.Contains(raw, "<reminder>Update your todos.</reminder>") {
		t.Fatalf("expected todo reminder in history, got:\n%s", raw)
	}
}

func messagesString(msgs []Message) string {
	var b strings.Builder
	for _, m := range msgs {
		b.WriteString(string(m.Role))
		b.WriteByte(' ')
		b.WriteString(m.Content)
		b.WriteByte('\n')
	}
	return b.String()
}
