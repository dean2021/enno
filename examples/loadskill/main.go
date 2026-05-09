package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dean2021/enno"
	openaiprovider "github.com/dean2021/enno/provider/openai"
	"github.com/dean2021/enno/sdk"
)

func main() {
	baseURL := mustEnv("ENNO_BASE_URL")
	model := mustEnv("ENNO_MODEL")

	root := os.Getenv("ENNO_SKILLS_DIR")
	if root == "" {
		// From repo root: go run ./examples/loadskill
		root = filepath.Join("examples", "skills")
	}

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
			{Name: "Identity", Content: "You are a skill-aware assistant."},
			{Name: "Rules", Content: "Use load_skill when a listed skill is relevant."},
		},
		BuiltinTools: sdk.BuiltinTools{
			LoadSkill: &sdk.LoadSkillTool{Dirs: []string{root}},
		},
	})
	if err != nil {
		panic(err)
	}

	result, err := agent.Run(context.Background(), &enno.Session{},
		"What skills exist? Load the sample skill and quote one requirement from it.")
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
