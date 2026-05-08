package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/dean2021/enno"
	anthropicprovider "github.com/dean2021/enno/provider/anthropic"
	"github.com/dean2021/enno/sdk"
)

func main() {
	model := mustEnv("ENNO_MODEL")

	provider, err := anthropicprovider.New(anthropicprovider.Config{
		APIKey:    os.Getenv("ANTHROPIC_API_KEY"),
		Model:     model,
		MaxTokens: 4096,
	})
	if err != nil {
		panic(err)
	}

	agent, err := sdk.NewAgent(sdk.Config{
		Provider:     provider,
		SystemPrompt: "You are a helpful agent.",
		BuiltinTools: sdk.BuiltinTools{
			TaskGraph: &sdk.TaskGraphTool{Root: ".", Timeout: 120 * time.Second},
		},
	})
	if err != nil {
		panic(err)
	}

	result, err := agent.Run(context.Background(), &enno.Session{}, "Make a short plan for learning Go.")
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
