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
	baseURL := mustEnv("ENNO_BASE_URL")
	model := mustEnv("ENNO_MODEL")

	tools := []enno.Tool{todo.New()}
	tools = append(tools, filesystem.New(filesystem.Config{Root: "."})...)

	agent, err := enno.NewAgent(enno.Config{
		Provider: openaiprovider.New(openaiprovider.Config{
			APIKey:  os.Getenv("ENNO_API_KEY"),
			BaseURL: baseURL,
			Model:   model,
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

func mustEnv(name string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	panic("missing required environment variable: " + name)
}
