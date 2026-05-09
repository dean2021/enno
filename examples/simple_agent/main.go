package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/dean2021/enno"
	openaiprovider "github.com/dean2021/enno/provider/openai"
	_ "github.com/dean2021/enno/setup"
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

	agent, err := enno.NewAgent(enno.Config{
		Provider:     provider,
		SystemPrompt: "Follow the application-provided sections below.",
		SystemPromptSections: []enno.SystemPromptSection{
			{Name: "Identity", Content: "You are a helpful coding agent."},
			{Name: "Output Style", Content: "Be concise and concrete."},
		},
		BuiltinTools: enno.BuiltinTools{
			TaskGraph:  &enno.TaskGraphTool{Root: ".", Timeout: 120 * time.Second},
			Filesystem: &enno.FilesystemTool{Root: "."},
		},
	})
	if err != nil {
		panic(err)
	}

	result, err := agent.Run(context.Background(), &enno.Session{}, "List the files in this workspace.")
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
