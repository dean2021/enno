package openai

import (
	"encoding/json"
	"testing"

	openaisdk "github.com/openai/openai-go/v3"
)

func TestOpenAIAssistantThinkingFromExtraJSON(t *testing.T) {
	const raw = `{"role":"assistant","content":"","reasoning":"First check files."}`
	var msg openaisdk.ChatCompletionMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := openAIAssistantThinking(msg); got != "First check files." {
		t.Fatalf("expected reasoning text, got %q", got)
	}
}

func TestOpenAIAssistantThinkingEmptyWhenAbsent(t *testing.T) {
	const raw = `{"role":"assistant","content":"hi"}`
	var msg openaisdk.ChatCompletionMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := openAIAssistantThinking(msg); got != "" {
		t.Fatalf("expected empty thinking, got %q", got)
	}
}
