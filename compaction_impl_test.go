package enno

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMicroCompactionKeepsRecentToolResults(t *testing.T) {
	msgs := []Message{
		UserMessage("hi"),
		AssistantMessage("", []ToolCall{{ID: "1", Name: "echo", Arguments: json.RawMessage(`{}`)}}),
		ToolMessage("1", strings.Repeat("x", 120)),
		AssistantMessage("", []ToolCall{{ID: "2", Name: "echo", Arguments: json.RawMessage(`{}`)}}),
		ToolMessage("2", strings.Repeat("y", 120)),
		AssistantMessage("", []ToolCall{{ID: "3", Name: "cat", Arguments: json.RawMessage(`{}`)}}),
		ToolMessage("3", strings.Repeat("z", 120)),
	}
	microCompact(msgs, 2, 100)
	if len(msgs[2].Content) >= 100 {
		t.Fatalf("expected first tool result compacted, got len %d", len(msgs[2].Content))
	}
	if msgs[2].Content != "[Previous: used echo]" {
		t.Fatalf("got %q", msgs[2].Content)
	}
	if len(msgs[4].Content) < 100 {
		t.Fatal("expected second tool still long")
	}
	if len(msgs[6].Content) < 100 {
		t.Fatal("expected third tool still long")
	}
}

func TestShouldAutoCompactionThreshold(t *testing.T) {
	req := Request{
		SystemPrompt: strings.Repeat("a", 4000),
		Messages:     []Message{UserMessage(strings.Repeat("b", 4000))},
	}
	if !shouldAutoCompact(req, 1000) {
		t.Fatal("expected over threshold")
	}
	if shouldAutoCompact(req, 1_000_000) {
		t.Fatal("expected under threshold")
	}
}
