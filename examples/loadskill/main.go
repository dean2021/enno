package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dean2021/enno"
	openaiprovider "github.com/dean2021/enno/provider/openai"
	"github.com/dean2021/enno/tools/loadskill"
)

func main() {
	baseURL := mustEnv("ENNO_BASE_URL")
	model := mustEnv("ENNO_MODEL")

	root := os.Getenv("ENNO_SKILLS_DIR")
	if root == "" {
		// From repo root: go run ./examples/loadskill
		root = filepath.Join("examples", "skills")
	}

	reg, err := loadskill.LoadDir(root)
	if err != nil {
		panic(err)
	}
	if reg.Count() == 0 {
		panic("no skills found under " + root + "; set ENNO_SKILLS_DIR or run from repo root")
	}

	loadTool, err := loadskill.NewTool(reg)
	if err != nil {
		panic(err)
	}

	sys := `You are a helpful assistant.
Skills available:
` + reg.DescriptionsText() + `
Use load_skill when you need full skill instructions.`

	agent, err := enno.NewAgent(enno.Config{
		Provider: openaiprovider.New(openaiprovider.Config{
			APIKey:  os.Getenv("ENNO_API_KEY"),
			BaseURL: baseURL,
			Model:   model,
		}),
		SystemPrompt: sys,
		Tools:        []enno.Tool{loadTool},
	})
	if err != nil {
		panic(err)
	}

	answer, err := agent.Run(context.Background(),
		"What skills exist? Load the sample skill and quote one requirement from it.")
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
