package enno

import (
	"context"
	"strings"
	"testing"
)

type compactionSummaryProvider struct {
	calls      int
	summarized string
}

func (p *compactionSummaryProvider) Complete(_ context.Context, req Request) (Response, error) {
	p.calls++
	if req.SystemPrompt == compactionSummarySystemPrompt {
		p.summarized = req.Messages[0].Content
		return Response{Content: "summary line"}, nil
	}
	return Response{Content: "ok", Usage: Usage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2}}, nil
}

func TestAgentAutoCompactionBeforeFirstModelCall(t *testing.T) {
	dir := t.TempDir()
	prov := &compactionSummaryProvider{}
	agent, err := NewAgent(Config{
		Provider: prov,
		Compaction: &CompactionConfig{
			Enabled:                true,
			TranscriptDir:          dir,
			AutoCompactInputTokens: 200,
		},
	})
	if err != nil {
		t.Fatalf("new agent: %v", err)
	}
	longInput := strings.Repeat("x", 900)
	session := Session{}
	result, err := agent.Run(context.Background(), &session, longInput)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Content != "ok" {
		t.Fatalf("answer: %q", result.Content)
	}
	if prov.calls != 2 {
		t.Fatalf("expected summarize then chat Complete calls, got %d", prov.calls)
	}
	if !strings.Contains(prov.summarized, longInput) {
		t.Fatal("summarize request should include prior user content")
	}
	msgs := session.Messages
	if len(msgs) != 2 {
		t.Fatalf("expected compressed user + assistant reply, got %d msgs", len(msgs))
	}
	if msgs[0].Role != RoleUser || !strings.HasPrefix(msgs[0].Content, "[Compressed]") {
		t.Fatalf("unexpected first message: %#v", msgs[0])
	}
}
