package main

import (
	"context"
	"fmt"
	"os"

	"github.com/dean2021/enno"
	"github.com/dean2021/enno/internal/cliconfig"
	"github.com/dean2021/enno/internal/cliui"
)

func main() {
	ctx := context.Background()
	config, err := cliconfig.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	agent, err := enno.NewAgent(config.AgentConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	switch config.Mode {
	case "run":
		answer, err := agent.Run(ctx, config.Query)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if answer != "" {
			fmt.Println(answer)
		}
	default:
		if err := cliui.REPL(ctx, agent, cliui.Config{Prompt: config.Prompt}); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}
}
