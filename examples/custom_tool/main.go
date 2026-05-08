package main

import (
	"context"
	"fmt"
	"os"

	"github.com/dean2021/enno"
	openaiprovider "github.com/dean2021/enno/provider/openai"
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

	agent, err := enno.NewAgent(enno.Config{
		Provider:     provider,
		SystemPrompt: "Use tools when they help.",
		Tools:        []enno.Tool{greet},
	})
	if err != nil {
		panic(err)
	}

	answer, err := agent.Run(context.Background(), "Please greet Dean.")
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
