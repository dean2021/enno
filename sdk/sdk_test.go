package sdk

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dean2021/enno"
)

type captureProvider struct {
	requests []enno.Request
	tool     string
}

func (p *captureProvider) Complete(ctx context.Context, req enno.Request) (enno.Response, error) {
	p.requests = append(p.requests, req)
	if p.tool != "" && len(req.Messages) == 1 {
		return enno.Response{ToolCalls: []enno.ToolCall{{
			ID:        "call_1",
			Name:      p.tool,
			Arguments: json.RawMessage(`{}`),
		}}}, nil
	}
	return enno.Response{Content: "done"}, nil
}

func TestNewAgentAssemblesConfiguredBuiltinsOnly(t *testing.T) {
	provider := &captureProvider{}
	agent, err := NewAgent(Config{
		Provider: provider,
		BuiltinTools: BuiltinTools{
			Filesystem: &FilesystemTool{Root: ".", Read: true},
			Grep:       &GrepTool{Root: "."},
			FetchURL:   &FetchURLTool{},
		},
	})
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}
	if _, err := agent.Run(context.Background(), &enno.Session{}, "hello"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	got := toolNames(provider.requests[0].Tools)
	want := []string{"read_file", "grep", "fetch_url"}
	if !sameStrings(got, want) {
		t.Fatalf("tools = %#v, want %#v", got, want)
	}
}

func TestNewAgentMergesCustomTools(t *testing.T) {
	provider := &captureProvider{}
	custom := enno.NewTool("custom_lookup", "Lookup.", nil, nil, func(context.Context, json.RawMessage) (string, error) {
		return "ok", nil
	})
	agent, err := NewAgent(Config{
		Provider:    provider,
		CustomTools: []enno.Tool{custom},
	})
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}
	if _, err := agent.Run(context.Background(), &enno.Session{}, "hello"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	got := toolNames(provider.requests[0].Tools)
	want := []string{"custom_lookup"}
	if !sameStrings(got, want) {
		t.Fatalf("tools = %#v, want %#v", got, want)
	}
}

func TestPermissionsAllowedToolsDenyUnlistedTool(t *testing.T) {
	provider := &captureProvider{tool: "bash"}
	bash := enno.NewTool("bash", "Run shell.", nil, nil, func(context.Context, json.RawMessage) (string, error) {
		return "should not run", nil
	})
	agent, err := NewAgent(Config{
		Provider:    provider,
		CustomTools: []enno.Tool{bash},
		Permissions: ToolPermissions{
			AllowedTools: []string{"read_file"},
		},
	})
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}
	result, err := agent.Run(context.Background(), &enno.Session{}, "hello")
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
	bash := enno.NewTool("bash", "Run shell.", nil, nil, func(context.Context, json.RawMessage) (string, error) {
		return "should not run", nil
	})
	agent, err := NewAgent(Config{
		Provider:    provider,
		CustomTools: []enno.Tool{bash},
		Permissions: ToolPermissions{
			AllowedTools:    []string{"bash"},
			DisallowedTools: []string{"bash"},
		},
	})
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}
	result, err := agent.Run(context.Background(), &enno.Session{}, "hello")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !result.Rounds[0].ToolCalls[0].Error {
		t.Fatalf("expected disallowed tool to be denied")
	}
}

func TestPermissionDenyWithoutAllowedToolsDeniesAll(t *testing.T) {
	provider := &captureProvider{tool: "custom_lookup"}
	custom := enno.NewTool("custom_lookup", "Lookup.", nil, nil, func(context.Context, json.RawMessage) (string, error) {
		return "should not run", nil
	})
	agent, err := NewAgent(Config{
		Provider:    provider,
		CustomTools: []enno.Tool{custom},
		Permissions: ToolPermissions{Mode: PermissionDeny},
	})
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}
	result, err := agent.Run(context.Background(), &enno.Session{}, "hello")
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

func TestSubagentInheritsPermissions(t *testing.T) {
	provider := &subagentPermissionProvider{}
	agent, err := NewAgent(Config{
		Provider:     provider,
		SystemPrompt: "parent",
		BuiltinTools: BuiltinTools{
			Shell:    &ShellTool{},
			Subagent: &SubagentTool{SystemPrompt: "child"},
		},
		Permissions: ToolPermissions{
			DisallowedTools: []string{"bash"},
		},
	})
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}
	result, err := agent.Run(context.Background(), &enno.Session{}, "delegate")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Content != "parent done" {
		t.Fatalf("content = %q", result.Content)
	}
	if provider.childToolResult != "Error: tool bash denied by permissions" {
		t.Fatalf("child tool result = %q", provider.childToolResult)
	}
	if provider.parentToolResult != "child done" {
		t.Fatalf("parent subagent result = %q", provider.parentToolResult)
	}
}

type subagentPermissionProvider struct {
	childToolResult  string
	parentToolResult string
}

func (p *subagentPermissionProvider) Complete(ctx context.Context, req enno.Request) (enno.Response, error) {
	last := req.Messages[len(req.Messages)-1]
	switch {
	case req.SystemPrompt == "parent" && last.Role == enno.RoleUser:
		return enno.Response{ToolCalls: []enno.ToolCall{{
			ID:        "subagent_call",
			Name:      "subagent",
			Arguments: json.RawMessage(`{"prompt":"try shell"}`),
		}}}, nil
	case req.SystemPrompt == "child" && last.Role == enno.RoleUser:
		return enno.Response{ToolCalls: []enno.ToolCall{{
			ID:        "bash_call",
			Name:      "bash",
			Arguments: json.RawMessage(`{"command":"printf child"}`),
		}}}, nil
	case req.SystemPrompt == "child" && last.Role == enno.RoleTool:
		p.childToolResult = last.Content
		return enno.Response{Content: "child done"}, nil
	case req.SystemPrompt == "parent" && last.Role == enno.RoleTool:
		p.parentToolResult = last.Content
		return enno.Response{Content: "parent done"}, nil
	default:
		return enno.Response{Content: "fallback"}, nil
	}
}

func toolNames(tools []enno.Tool) []string {
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
