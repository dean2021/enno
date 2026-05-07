package main

import (
	"context"
	"fmt"
	"os"

	"github.com/dean2021/enno"
	anthropicprovider "github.com/dean2021/enno/provider/anthropic"
	"github.com/dean2021/enno/tools/todo"
)

func main() {
	model := os.Getenv("ENNO_MODEL")
	if model == "" {
		model = "claude-sonnet-4-5-20250929"
	}

	agent, err := enno.NewAgent(enno.Config{
		Provider: anthropicprovider.New(anthropicprovider.Config{
			APIKey:    os.Getenv("ANTHROPIC_API_KEY"),
			Model:     model,
			MaxTokens: 4096,
		}),
		SystemPrompt: "You are a helpful agent.",
		Tools:        []enno.Tool{todo.New()},
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
