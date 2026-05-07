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
	greet := enno.NewTypedTool("greet", "Greet a person by name.", map[string]any{
		"name": map[string]any{"type": "string"},
	}, []string{"name"}, func(ctx context.Context, args greetArgs) (string, error) {
		return fmt.Sprintf("Hello, %s!", args.Name), nil
	})

	agent, err := enno.NewAgent(enno.Config{
		Provider: openaiprovider.New(openaiprovider.Config{
			APIKey:  os.Getenv("ENNO_API_KEY"),
			BaseURL: envOr("ENNO_BASE_URL", "https://maas-coding-api.cn-huabei-1.xf-yun.com/v2"),
			Model:   envOr("ENNO_MODEL", "astron-code-latest"),
		}),
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

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
