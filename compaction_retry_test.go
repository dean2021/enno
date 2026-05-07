package enno

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type retrySummarizeProvider struct {
	calls int
}

func (p *retrySummarizeProvider) Complete(_ context.Context, _ Request) (Response, error) {
	p.calls++
	if p.calls == 1 {
		return Response{}, errors.New("fail first")
	}
	return Response{Content: "<summary>ok</summary>"}, nil
}

func TestSummarizeCompactionRetriesOnFailure(t *testing.T) {
	p := &retrySummarizeProvider{}
	msgs := []Message{UserMessage("a"), UserMessage("b")}
	out, err := summarizeCompaction(context.Background(), p, msgs)
	if err != nil {
		t.Fatal(err)
	}
	if p.calls != 2 {
		t.Fatalf("expected 2 Complete calls, got %d", p.calls)
	}
	if !strings.Contains(out, "ok") {
		t.Fatalf("out: %q", out)
	}
}
