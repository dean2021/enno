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
	microCompact(msgs, 2, 100, nil)
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

func TestMicroCompactionWhitelistOnlyNamedTools(t *testing.T) {
	msgs := []Message{
		UserMessage("hi"),
		AssistantMessage("", []ToolCall{{ID: "1", Name: "echo", Arguments: json.RawMessage(`{}`)}}),
		ToolMessage("1", strings.Repeat("x", 120)),
		AssistantMessage("", []ToolCall{{ID: "2", Name: "echo", Arguments: json.RawMessage(`{}`)}}),
		ToolMessage("2", strings.Repeat("y", 120)),
		AssistantMessage("", []ToolCall{{ID: "3", Name: "todo", Arguments: json.RawMessage(`{}`)}}),
		ToolMessage("3", strings.Repeat("z", 120)),
	}
	microCompact(msgs, 1, 100, []string{"echo"})
	if msgs[2].Content != "[Previous: used echo]" {
		t.Fatalf("first echo should compact: %q", msgs[2].Content)
	}
	if len(msgs[4].Content) < 100 {
		t.Fatal("most recent echo should stay long")
	}
	if len(msgs[6].Content) < 100 {
		t.Fatal("todo not whitelisted should stay full length")
	}
}

func TestInputTokensOverThreshold(t *testing.T) {
	req := Request{
		SystemPrompt: strings.Repeat("a", 4000),
		Messages:     []Message{UserMessage(strings.Repeat("b", 4000))},
	}
	cfg := CompactionConfig{AutoCompactInputTokens: 1000}
	if !inputTokensOverThreshold(req, cfg, 0) {
		t.Fatal("expected over threshold")
	}
	cfg.AutoCompactInputTokens = 1_000_000
	if inputTokensOverThreshold(req, cfg, 0) {
		t.Fatal("expected under threshold")
	}
}

func TestEffectiveThresholdModelWindow(t *testing.T) {
	cfg := CompactionConfig{
		ModelContextTokens:      100_000,
		AutoCompactBufferTokens: 13_000,
		AutoCompactInputTokens:  9999,
	}
	if g, w := effectiveAutoCompactThreshold(cfg), int64(87_000); g != w {
		t.Fatalf("got %d want %d", g, w)
	}
}

func TestFormatCompactSummary(t *testing.T) {
	raw := `<analysis>think</analysis>
<summary>
line1
</summary>
trailing`
	out := FormatCompactSummary(raw)
	if strings.Contains(out, "analysis") || strings.Contains(out, "<summary>") {
		t.Fatalf("unexpected: %q", out)
	}
	if !strings.HasPrefix(out, "Summary:") {
		t.Fatalf("got %q", out)
	}
	plain := "no tags at all"
	if FormatCompactSummary(plain) != "no tags at all" {
		t.Fatalf("plain text should pass through: %q", FormatCompactSummary(plain))
	}
}
