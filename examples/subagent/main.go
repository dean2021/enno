package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/dean2021/enno"
	openaiprovider "github.com/dean2021/enno/provider/openai"
	"github.com/dean2021/enno/sdk"
)

func main() {
	baseURL := mustEnv("ENNO_BASE_URL")
	model := mustEnv("ENNO_MODEL")

	provider, err := openaiprovider.New(openaiprovider.Config{
		APIKey:  os.Getenv("ENNO_API_KEY"),
		BaseURL: baseURL,
		Model:   model,
	})
	if err != nil {
		panic(err)
	}

	agent, err := sdk.NewAgent(sdk.Config{
		Provider:     provider,
		SystemPrompt: "Follow the application-provided sections below.",
		SystemPromptSections: []sdk.SystemPromptSection{
			{Name: "Identity", Content: "You are a helpful coding agent."},
			{Name: "Rules", Content: "You may use the subagent tool to delegate exploration with a fresh context; only the child agent's final reply is returned here."},
		},
		BuiltinTools: sdk.BuiltinTools{
			TaskGraph:  &sdk.TaskGraphTool{Root: ".", Timeout: 120 * time.Second},
			Filesystem: &sdk.FilesystemTool{Root: "."},
			Subagent:   &sdk.SubagentTool{},
		},
	})
	if err != nil {
		panic(err)
	}

	result, err := agent.Run(context.Background(), &enno.Session{},
		"Use the subagent tool once to list Go files under . then summarize in one sentence.")
	if err != nil {
		panic(err)
	}
	fmt.Println(result.Content)
}

func mustEnv(name string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	panic("missing required environment variable: " + name)
}
