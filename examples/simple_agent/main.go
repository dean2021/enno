package main

import (
	"context"
	"fmt"
	"os"

	"github.com/dean2021/enno"
	openaiprovider "github.com/dean2021/enno/provider/openai"
	"github.com/dean2021/enno/tools/filesystem"
	"github.com/dean2021/enno/tools/todo"
)

func main() {
	tools := []enno.Tool{todo.New()}
	tools = append(tools, filesystem.New(filesystem.Config{Root: "."})...)

	agent, err := enno.NewAgent(enno.Config{
		Provider: openaiprovider.New(openaiprovider.Config{
			APIKey:  os.Getenv("ENNO_API_KEY"),
			BaseURL: envOr("ENNO_BASE_URL", "https://maas-coding-api.cn-huabei-1.xf-yun.com/v2"),
			Model:   envOr("ENNO_MODEL", "astron-code-latest"),
		}),
		SystemPrompt: "You are a helpful coding agent.",
		Tools:        tools,
	})
	if err != nil {
		panic(err)
	}

	answer, err := agent.Run(context.Background(), "List the files in this workspace.")
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
