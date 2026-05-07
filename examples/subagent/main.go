package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/dean2021/enno"
	openaiprovider "github.com/dean2021/enno/provider/openai"
	"github.com/dean2021/enno/tools/filesystem"
	"github.com/dean2021/enno/tools/subagent"
	"github.com/dean2021/enno/tools/taskgraph"
)

func main() {
	baseURL := mustEnv("ENNO_BASE_URL")
	model := mustEnv("ENNO_MODEL")

	provider := openaiprovider.New(openaiprovider.Config{
		APIKey:  os.Getenv("ENNO_API_KEY"),
		BaseURL: baseURL,
		Model:   model,
	})

	childTools := taskgraph.New(taskgraph.Config{Root: ".", Timeout: 120 * time.Second})
	childTools = append(childTools, filesystem.New(filesystem.Config{Root: "."})...)

	subagentTool, err := subagent.New(subagent.Config{
		Provider:   provider,
		ChildTools: childTools,
	})
	if err != nil {
		panic(err)
	}

	parentTools := append(append([]enno.Tool(nil), childTools...), subagentTool)

	agent, err := enno.NewAgent(enno.Config{
		Provider: provider,
		SystemPrompt: `You are a helpful coding agent. You may use the subagent tool to delegate exploration with a fresh context;
only the child agent's final reply is returned here.`,
		Tools: parentTools,
	})
	if err != nil {
		panic(err)
	}

	answer, err := agent.Run(context.Background(),
		"Use the subagent tool once to list Go files under . then summarize in one sentence.")
	if err != nil {
		panic(err)
	}
	fmt.Println(answer)
}

func mustEnv(name string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	panic("missing required environment variable: " + name)
}
