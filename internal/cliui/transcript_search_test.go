package cliui

import (
	"strings"
	"testing"
	"time"

	"github.com/dean2021/enno"
)

func TestPlainTextForSearchMirrorsMessages(t *testing.T) {
	s := newMainViewState()
	s.AppendMessage("you", "hello")
	s.AppendMessage("enno", "world")
	s.AppendRichMessage("tool", `[aqua]bash[white]([purple]"x"[white])`)

	plain := plainTextForSearch(s)
	if !strings.Contains(plain, "You: hello") {
		t.Fatalf("missing user line: %q", plain)
	}
	if !strings.Contains(plain, "Enno: world") {
		t.Fatalf("missing assistant line: %q", plain)
	}
	if strings.Contains(plain, "[aqua]") {
		t.Fatalf("plain text should strip tags: %q", plain)
	}
}

func TestLineOfFirstMatch(t *testing.T) {
	plain := "You: hello\n\nEnno: world reply\n\n"
	line, ok := lineOfFirstMatch(plain, "world")
	if !ok || line != 2 {
		t.Fatalf("got line=%d ok=%v want line=2", line, ok)
	}
	_, ok = lineOfFirstMatch(plain, "nomatch")
	if ok {
		t.Fatal("expected no match")
	}
	_, ok = lineOfFirstMatch(plain, "")
	if ok {
		t.Fatal("empty query should not match")
	}
}

func TestPlainTextWithRichEventLines(t *testing.T) {
	s := newMainViewState()
	s.AppendEvent(enno.Event{
		Type:     enno.EventModelResponse,
		Round:    1,
		Thinking: "hidden thought",
		Usage:    enno.Usage{},
		Duration: time.Second,
	})

	plain := plainTextForSearch(s)
	if !strings.Contains(plain, "Thinking") && !strings.Contains(strings.ToLower(plain), "thinking") {
		t.Fatalf("expected thinking visible in plain: %q", plain)
	}
	line, ok := lineOfFirstMatch(plain, "thought")
	if !ok {
		t.Fatalf("expected match in %q", plain)
	}
	if line < 0 {
		t.Fatalf("line %d", line)
	}
}
