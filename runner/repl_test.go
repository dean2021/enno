package runner

import (
	"context"
	"strings"
	"testing"

	"github.com/dean2021/enno"
)

type stubProvider struct {
	calls int
}

func (p *stubProvider) Complete(_ context.Context, req enno.Request) (enno.Response, error) {
	p.calls++
	last := req.Messages[len(req.Messages)-1]
	return enno.Response{Content: "answer: " + last.Content}, nil
}

func TestREPLPlainFallbackRunsPrompt(t *testing.T) {
	provider := &stubProvider{}
	agent, err := enno.NewAgent(enno.Config{Provider: provider})
	if err != nil {
		t.Fatalf("new agent: %v", err)
	}

	var out strings.Builder
	err = REPL(context.Background(), agent, Config{
		In:  strings.NewReader("hello\nexit\n"),
		Out: &out,
	})
	if err != nil {
		t.Fatalf("repl: %v", err)
	}

	if provider.calls != 1 {
		t.Fatalf("expected 1 provider call, got %d", provider.calls)
	}
	if !strings.Contains(out.String(), "answer: hello") {
		t.Fatalf("expected answer in output, got %q", out.String())
	}
}

func TestREPLPlainFallbackSkipsEmptyInputAndExits(t *testing.T) {
	provider := &stubProvider{}
	agent, err := enno.NewAgent(enno.Config{Provider: provider})
	if err != nil {
		t.Fatalf("new agent: %v", err)
	}

	err = REPL(context.Background(), agent, Config{
		In: strings.NewReader("\nq\n"),
	})
	if err != nil {
		t.Fatalf("repl: %v", err)
	}
	if provider.calls != 0 {
		t.Fatalf("expected no provider calls, got %d", provider.calls)
	}
}
