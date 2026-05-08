package main

import (
	"context"
	"fmt"
	"os"

	"github.com/dean2021/enno"
	"github.com/dean2021/enno/internal/cliconfig"
	"github.com/dean2021/enno/internal/cliui"
	"github.com/dean2021/enno/internal/history"
)

func main() {
	ctx := context.Background()
	config, err := cliconfig.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	var events chan enno.Event
	if config.Mode != "run" {
		events = make(chan enno.Event, 256)
		config.AgentConfig.EventHandler = func(_ context.Context, event enno.Event) {
			select {
			case events <- event:
			default:
				fmt.Fprintf(os.Stderr, "[enno] event dropped: %s\n", event.Type)
			}
		}
	}

	agent, err := enno.NewAgent(config.AgentConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	session := &enno.Session{}

	histPath, err := history.DefaultPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	recorder, err := history.NewRecorder(histPath, config.Project, config.SessionID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer recorder.Close()

	switch config.Mode {
	case "run":
		_ = recorder.Record(config.Query)
		result, err := agent.Run(ctx, session, config.Query)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if result.Content != "" {
			fmt.Println(result.Content)
		}
	default:
		if err := cliui.REPL(ctx, agent, cliui.Config{
			Prompt:       config.Prompt,
			Session:      session,
			Events:       events,
			Recorder:     recorder,
			DisableMouse: config.DisableMouse,
		}); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}
}
