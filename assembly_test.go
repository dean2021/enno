package enno

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestAssembleConfigComposesSystemPromptSections(t *testing.T) {
	cfg, err := AssembleConfig(Config{
		Provider:     &captureProvider{},
		SystemPrompt: "Base prompt.",
		SystemPromptSections: []SystemPromptSection{
			{Name: "Identity", Content: "You are a test agent."},
			{Name: "Rules", Content: "Answer briefly."},
			{Name: "Empty", Content: "  "},
		},
	})
	if err != nil {
		t.Fatalf("AssembleConfig: %v", err)
	}
	want := "Base prompt.\n\n# Identity\nYou are a test agent.\n\n# Rules\nAnswer briefly."
	if cfg.SystemPrompt != want {
		t.Fatalf("SystemPrompt = %q, want %q", cfg.SystemPrompt, want)
	}
}

func TestAssembleConfigAppendsSkillsAfterCustomSections(t *testing.T) {
	cfg := assembleSystemPrompt("Base prompt.", []SystemPromptSection{
		{Name: "Identity", Content: "You are a test agent."},
	}, "  - demo: test skill")
	want := "Base prompt.\n\n# Identity\nYou are a test agent.\n\n# Skills\nSkills available:\n- demo: test skill\nCall load_skill with a skill name when you need the full instructions for that workflow."
	if cfg != want {
		t.Fatalf("system prompt = %q, want %q", cfg, want)
	}
	for _, notWant := range []string{"# Doing Tasks", "# Safety", "# Environment", "# Project Instructions", "# Tool Guidance", "# Communication"} {
		if strings.Contains(cfg, notWant) {
			t.Fatalf("runtime prompt should not contain coding-agent section %q:\n%s", notWant, cfg)
		}
	}
}

func TestAssembleConfigDoesNotInjectDefaultIdentity(t *testing.T) {
	cfg, err := AssembleConfig(Config{
		Provider:     &captureProvider{},
		SystemPrompt: "Base prompt.",
	})
	if err != nil {
		t.Fatalf("AssembleConfig: %v", err)
	}
	if cfg.SystemPrompt != "Base prompt." {
		t.Fatalf("SystemPrompt = %q", cfg.SystemPrompt)
	}
}

func TestPermissionsAllowedToolsDenyUnlistedTool(t *testing.T) {
	provider := &captureProvider{tool: "bash"}
	bash := NewTool("bash", "Run shell.", nil, nil, func(context.Context, json.RawMessage) (string, error) {
		return "should not run", nil
	})
	agent, err := NewAgent(Config{
		Provider:    provider,
		Tools:       []Tool{bash},
		Permissions: ToolPermissions{AllowedTools: []string{"read_file"}},
	})
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}
	result, err := agent.Run(context.Background(), &Session{}, "hello")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	call := result.Rounds[0].ToolCalls[0]
	if !call.Error || call.Result != "Error: tool bash denied by permissions" {
		t.Fatalf("tool result = %#v", call)
	}
}

func TestPermissionsDisallowedWinsOverAllowed(t *testing.T) {
	provider := &captureProvider{tool: "bash"}
	bash := NewTool("bash", "Run shell.", nil, nil, func(context.Context, json.RawMessage) (string, error) {
		return "should not run", nil
	})
	agent, err := NewAgent(Config{
		Provider:    provider,
		Tools:       []Tool{bash},
		Permissions: ToolPermissions{AllowedTools: []string{"bash"}, DisallowedTools: []string{"bash"}},
	})
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}
	result, err := agent.Run(context.Background(), &Session{}, "hello")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !result.Rounds[0].ToolCalls[0].Error {
		t.Fatalf("expected disallowed tool to be denied")
	}
}

func TestPermissionDenyWithoutAllowedToolsDeniesAll(t *testing.T) {
	provider := &captureProvider{tool: "custom_lookup"}
	custom := NewTool("custom_lookup", "Lookup.", nil, nil, func(context.Context, json.RawMessage) (string, error) {
		return "should not run", nil
	})
	agent, err := NewAgent(Config{
		Provider:    provider,
		Tools:       []Tool{custom},
		Permissions: ToolPermissions{Mode: PermissionDeny},
	})
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}
	result, err := agent.Run(context.Background(), &Session{}, "hello")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !result.Rounds[0].ToolCalls[0].Error {
		t.Fatalf("expected deny mode to deny all tools without allow list")
	}
}

func TestInvalidPermissionModeFails(t *testing.T) {
	_, err := NewAgent(Config{
		Provider:    &captureProvider{},
		Permissions: ToolPermissions{Mode: PermissionMode("sometimes")},
	})
	if err == nil {
		t.Fatal("expected invalid permission mode error")
	}
}

func TestNewAgentWithoutToolBuilderReturnsErrorForBuiltinTools(t *testing.T) {
	toolBuilder = nil
	defer func() { toolBuilder = nil }()
	_, err := NewAgent(Config{
		Provider:     &captureProvider{},
		BuiltinTools: BuiltinTools{Grep: &GrepTool{Root: "."}},
	})
	if err == nil {
		t.Fatal("expected error when no tool builder is registered")
	}
	if !strings.Contains(err.Error(), "no ToolBuilder registered") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewAgentWithoutToolBuilderWorksWithDirectTools(t *testing.T) {
	toolBuilder = nil
	defer func() { toolBuilder = nil }()
	provider := &captureProvider{}
	agent, err := NewAgent(Config{
		Provider: provider,
		Tools:    []Tool{NewTool("test", "A test tool.", nil, nil, func(context.Context, json.RawMessage) (string, error) { return "ok", nil })},
	})
	if err != nil {
		t.Fatalf("NewAgent with direct Tools should not require ToolBuilder: %v", err)
	}
	if _, err := agent.Run(context.Background(), &Session{}, "hello"); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

type captureProvider struct {
	requests []Request
	tool     string
}

func (p *captureProvider) Complete(ctx context.Context, req Request) (Response, error) {
	p.requests = append(p.requests, req)
	if p.tool != "" && len(req.Messages) == 1 {
		return Response{ToolCalls: []ToolCall{{
			ID:        "call_1",
			Name:      p.tool,
			Arguments: json.RawMessage(`{}`),
		}}}, nil
	}
	return Response{Content: "done"}, nil
}

func toolNames(tools []Tool) []string {
	names := make([]string, len(tools))
	for i, tool := range tools {
		names[i] = tool.Name
	}
	return names
}

func sameStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
