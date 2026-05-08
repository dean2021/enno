package cliui

import (
	"strings"
	"testing"
)

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

func TestStripANSI(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"\x1b[31mhello\x1b[0m", "hello"},
		{"\x1b[38;2;146;181;253mYou:\x1b[0m run tests", "You: run tests"},
		{"no escapes here", "no escapes here"},
		{"\x1b[1mbold\x1b[0m \x1b[38;2;56;189;248mtool\x1b[0m", "bold tool"},
	}
	for _, tt := range tests {
		got := stripANSI(tt.input)
		if got != tt.want {
			t.Errorf("stripANSI(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSearchUsesRenderedContent(t *testing.T) {
	state := newMainViewState()
	state.AppendMessage("you", "findme")
	state.AppendMessage("enno", "answer here")

	rendered := state.ViewportString(80)
	plain := stripANSI(rendered)
	line, ok := lineOfFirstMatch(plain, "findme")
	if !ok {
		t.Fatalf("expected to find 'findme' in plain content: %q", plain)
	}
	vpContent := strings.Split(plain, "\n")
	if line >= len(vpContent) {
		t.Fatalf("line %d out of range (total %d lines)", line, len(vpContent))
	}
	if !strings.Contains(vpContent[line], "findme") {
		t.Fatalf("line %d does not contain 'findme': %q", line, vpContent[line])
	}
}
