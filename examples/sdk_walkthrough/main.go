package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dean2021/enno"
	"github.com/dean2021/enno/sdk"
)

type workspaceStatsArgs struct {
	Root        string `json:"root"`
	IncludeDocs bool   `json:"include_docs"`
}

func main() {
	ctx := context.Background()

	statsSchema := enno.SchemaObject().
		StringProp("root").
		BooleanProp("include_docs").
		Required("root")

	workspaceStats := enno.NewStructuredTool("workspace_stats", "Summarize repository layout.", statsSchema.Properties(), statsSchema.RequiredFields(),
		func(ctx context.Context, raw json.RawMessage) (enno.ToolResult, error) {
			var args workspaceStatsArgs
			if err := json.Unmarshal(raw, &args); err != nil {
				return enno.ToolResult{}, err
			}
			content := fmt.Sprintf("root=%s packages=12 docs=%t examples=6", args.Root, args.IncludeDocs)
			return enno.ToolResult{
				Content: content,
				Metadata: map[string]any{
					"root":          args.Root,
					"package_count": 12,
					"doc_coverage":  args.IncludeDocs,
				},
			}, nil
		})

	temperature := 0.2
	agent, err := sdk.NewAgent(sdk.Config{
		Provider:     scriptedProvider{},
		SystemPrompt: "Follow the application-provided sections below.",
		SystemPromptSections: []sdk.SystemPromptSection{
			{Name: "Identity", Content: "You are a concise SDK tutorial agent."},
			{Name: "Rules", Content: "Use tools when they provide concrete repository facts."},
			{Name: "Output Style", Content: "Keep answers short and focused on SDK concepts."},
		},
		CustomTools: []enno.Tool{workspaceStats},
		Options: enno.RequestOptions{
			Temperature:     &temperature,
			MaxOutputTokens: 512,
			ToolChoice:      enno.ToolChoice{Type: enno.ToolChoiceAuto},
			Metadata:        map[string]string{"example": "sdk_walkthrough"},
		},
		Hooks:        []enno.Hook{auditHook{}},
		EventHandler: logEvent,
	})
	if err != nil {
		panic(err)
	}

	session := &enno.Session{}
	result, err := agent.Run(ctx, session, "Summarize this SDK example and inspect the repository shape.")
	if err != nil {
		panic(err)
	}

	fmt.Println("\n=== Final Answer ===")
	fmt.Println(result.Content)
	fmt.Printf("\nstop=%s rounds=%d messages=%d usage=%+v\n",
		result.StopReason, len(result.Rounds), len(result.Messages), result.Usage)
	printToolResults(result)

	branch := session.Clone()
	followUp, err := agent.Run(ctx, &branch, "What context is still available in this cloned session?")
	if err != nil {
		panic(err)
	}
	fmt.Println("\n=== Follow-up On Cloned Session ===")
	fmt.Println(followUp.Content)
	fmt.Printf("original_messages=%d cloned_messages=%d\n", len(session.Messages), len(branch.Messages))
}

type scriptedProvider struct{}

func (scriptedProvider) Complete(ctx context.Context, req enno.Request) (enno.Response, error) {
	if len(req.Messages) == 0 {
		return enno.Response{Content: "No messages were provided."}, nil
	}

	last := req.Messages[len(req.Messages)-1]
	if last.Role == enno.RoleUser && strings.Contains(strings.ToLower(last.Content), "inspect") {
		return enno.Response{
			ToolCalls: []enno.ToolCall{{
				ID:        "call_workspace_stats",
				Name:      "workspace_stats",
				Arguments: json.RawMessage(`{"root":".","include_docs":true}`),
			}},
			Usage: enno.Usage{InputTokens: 120, OutputTokens: 24},
		}, nil
	}

	if last.Role == enno.RoleTool {
		return enno.Response{
			Content: fmt.Sprintf("The SDK flow is: create a Provider, register Tools, create an Agent, pass an explicit Session, then read RunResult. Tool output was: %s. Request metadata example=%s.",
				last.Content, req.Options.Metadata["example"]),
			Usage: enno.Usage{InputTokens: 170, OutputTokens: 52},
		}, nil
	}

	if last.Role == enno.RoleUser {
		users, assistants, tools := countRoles(req.Messages)
		return enno.Response{
			Content: fmt.Sprintf("This request received %d messages: users=%d assistants=%d tools=%d. Because the run used Session.Clone, the original session was not changed.",
				len(req.Messages), users, assistants, tools),
			Usage: enno.Usage{InputTokens: 90, OutputTokens: 36},
		}, nil
	}

	return enno.Response{Content: "Done."}, nil
}

type auditHook struct {
	enno.NoopHook
}

func (auditHook) BeforeToolCall(ctx context.Context, state enno.BeforeToolCallState) (enno.BeforeToolCallResult, error) {
	fmt.Printf("[hook] allowing tool=%s round=%d\n", state.ToolCall.Name, state.Round)
	return enno.BeforeToolCallResult{}, nil
}

func logEvent(ctx context.Context, event enno.Event) {
	switch event.Type {
	case enno.EventModelStart, enno.EventRoundComplete:
		fmt.Printf("[event] %s round=%d messages=%d\n", event.Type, event.Round, event.MessageCount)
	case enno.EventToolStart:
		fmt.Printf("[event] %s round=%d tool=%s\n", event.Type, event.Round, event.ToolCall.Name)
	case enno.EventToolResult:
		fmt.Printf("[event] %s round=%d result=%q metadata=%v\n", event.Type, event.Round, event.ToolResult, event.ToolMetadata)
	}
}

func printToolResults(result enno.RunResult) {
	for _, round := range result.Rounds {
		for _, call := range round.ToolCalls {
			fmt.Printf("tool=%s result=%q metadata=%v duration=%s\n",
				call.Call.Name, call.Result, call.Metadata, call.Duration)
		}
	}
}

func countRoles(messages []enno.Message) (users int, assistants int, tools int) {
	for _, message := range messages {
		switch message.Role {
		case enno.RoleUser:
			users++
		case enno.RoleAssistant:
			assistants++
		case enno.RoleTool:
			tools++
		}
	}
	return users, assistants, tools
}
