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

func TestAgent_NoTaskGraphTools_DoesNotInjectReminder(t *testing.T) {
	p := &streakEchoProvider{streakRounds: 4}
	agent, err := NewAgent(Config{
		Provider: p,
		Tools:    []Tool{echoTool()},
	})
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}
	session := Session{}
	if _, err := agent.Run(context.Background(), &session, "start"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	raw := messagesString(session.Messages)
	if strings.Contains(raw, "<reminder>Update your task plan.</reminder>") {
		t.Fatalf("did not expect task plan reminder without task graph tools, messages:\n%s", raw)
	}
}

// stubTaskUpdateTool registers task_update without importing tools/*.
func stubTaskUpdateTool() Tool {
	return NewTool("task_update", "stub", map[string]any{
		"task_id": map[string]any{"type": "integer"},
	}, []string{"task_id"}, func(_ context.Context, _ json.RawMessage) (string, error) {
		return "ok", nil
	})
}

func TestAgent_WithTaskGraphTool_InjectsReminderAfterThreeToolRoundsWithoutPlanUpdate(t *testing.T) {
	p := &streakEchoProvider{streakRounds: 4}
	agent, err := NewAgent(Config{
		Provider: p,
		Tools:    []Tool{echoTool(), stubTaskUpdateTool()},
	})
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}
	session := Session{}
	if _, err := agent.Run(context.Background(), &session, "start"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	raw := messagesString(session.Messages)
	if !strings.Contains(raw, "<reminder>Update your task plan.</reminder>") {
		t.Fatalf("expected task plan reminder in history, got:\n%s", raw)
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
