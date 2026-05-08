package shell

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestShellHonorsMaxOutputChars(t *testing.T) {
	tool := New(Config{
		Timeout:        time.Second,
		MaxOutputChars: 30,
	})
	sh := tool.StructuredHandler
	result, err := sh(context.Background(), []byte(`{"command":"printf '%s' 'abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz'"}`))
	if err != nil {
		t.Fatalf("shell: %v", err)
	}
	if len([]rune(result.Content)) > 30 {
		t.Fatalf("output len = %d, want <= 30", len([]rune(result.Content)))
	}
	if !strings.Contains(result.Content, "[truncated]") {
		t.Fatalf("expected truncation suffix, got %q", result.Content)
	}
}

func TestShellSafetyPolicyAllowAllBypassesDenyList(t *testing.T) {
	s := &Shell{
		timeout:        time.Second,
		denyList:       []string{"echo"},
		maxOutputChars: 100,
		safetyPolicy:   SafetyPolicyAllowAll,
	}
	out, err := s.Run(context.Background(), "echo ok")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out != "ok" {
		t.Fatalf("out = %q, want ok", out)
	}
}
