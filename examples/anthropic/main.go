package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/dean2021/enno"
	anthropicprovider "github.com/dean2021/enno/provider/anthropic"
	"github.com/dean2021/enno/tools/taskgraph"
)

func main() {
	model := mustEnv("ENNO_MODEL")

	agent, err := enno.NewAgent(enno.Config{
		Provider: anthropicprovider.New(anthropicprovider.Config{
			APIKey:    os.Getenv("ANTHROPIC_API_KEY"),
			Model:     model,
			MaxTokens: 4096,
		}),
		SystemPrompt: "You are a helpful agent.",
		Tools:        taskgraph.New(taskgraph.Config{Root: ".", Timeout: 120 * time.Second}),
	})
	if err != nil {
		panic(err)
	}

	answer, err := agent.Run(context.Background(), "Make a short plan for learning Go.")
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
