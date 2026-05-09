package main

import (
	"context"
	"fmt"
	"os"

	"github.com/dean2021/enno"
	openaiprovider "github.com/dean2021/enno/provider/openai"
	"github.com/dean2021/enno/sdk"
)

type greetArgs struct {
	Name string `json:"name"`
}

func main() {
	baseURL := mustEnv("ENNO_BASE_URL")
	model := mustEnv("ENNO_MODEL")

	greet := enno.NewTypedTool("greet", "Greet a person by name.", map[string]any{
		"name": map[string]any{"type": "string"},
	}, []string{"name"}, func(ctx context.Context, args greetArgs) (string, error) {
		return fmt.Sprintf("Hello, %s!", args.Name), nil
	})

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
			{Name: "Identity", Content: "You are a tool-using assistant."},
			{Name: "Rules", Content: "Use tools when they help and keep replies short."},
		},
		CustomTools: []enno.Tool{greet},
	})
	if err != nil {
		panic(err)
	}

	result, err := agent.Run(context.Background(), &enno.Session{}, "Please greet Dean.")
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
