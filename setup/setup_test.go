package setup_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/dean2021/enno"

	_ "github.com/dean2021/enno/setup"
)

func TestNewAgentAssemblesConfiguredBuiltinsOnly(t *testing.T) {
	provider := &stubProvider{}
	agent, err := enno.NewAgent(enno.Config{
		Provider: provider,
		BuiltinTools: enno.BuiltinTools{
			Filesystem: &enno.FilesystemTool{Root: ".", Read: true},
			Grep:       &enno.GrepTool{Root: "."},
			FetchURL:   &enno.FetchURLTool{},
		},
	})
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}
	if _, err := agent.Run(context.Background(), &enno.Session{}, "hello"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	got := providerToolNames(provider)
	want := []string{"read_file", "grep", "fetch_url"}
	if !sameStrings(got, want) {
		t.Fatalf("tools = %#v, want %#v", got, want)
	}
}

func TestNewAgentMergesCustomAndBuiltinTools(t *testing.T) {
	provider := &stubProvider{}
	custom := enno.NewTool("custom_lookup", "Lookup.", nil, nil, func(context.Context, json.RawMessage) (string, error) {
		return "ok", nil
	})
	agent, err := enno.NewAgent(enno.Config{
		Provider:    provider,
		CustomTools: []enno.Tool{custom},
		BuiltinTools: enno.BuiltinTools{
			TaskGraph: &enno.TaskGraphTool{Root: ".", Timeout: 120 * time.Second},
		},
	})
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}
	if _, err := agent.Run(context.Background(), &enno.Session{}, "hello"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	got := providerToolNames(provider)
	found := false
	for _, name := range got {
		if name == "custom_lookup" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("custom_lookup not found in tools: %v", got)
	}
	found = false
	for _, name := range got {
		if name == "task_create" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("task_create not found in tools: %v", got)
	}
}

type stubProvider struct {
	requests []enno.Request
}

func (p *stubProvider) Complete(ctx context.Context, req enno.Request) (enno.Response, error) {
	p.requests = append(p.requests, req)
	return enno.Response{Content: "done"}, nil
}

func providerToolNames(p *stubProvider) []string {
	if len(p.requests) == 0 {
		return nil
	}
	names := make([]string, len(p.requests[0].Tools))
	for i, tool := range p.requests[0].Tools {
		names[i] = tool.Name
	}
	return names
}

func sameStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
outer:
	for _, w := range want {
		for _, g := range got {
			if g == w {
				continue outer
			}
		}
		return false
	}
	return true
}
