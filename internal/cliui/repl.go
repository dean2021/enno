package cliui

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dean2021/enno"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type Config struct {
	Prompt string
	In     io.Reader
	Out    io.Writer
	Err    io.Writer
}

func REPL(ctx context.Context, agent *enno.Agent, config Config) error {
	config = config.withDefaults()
	if !isTerminal(config.In) {
		return plainREPL(ctx, agent, config)
	}
	return tuiREPL(ctx, agent, config)
}

func tuiREPL(ctx context.Context, agent *enno.Agent, config Config) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	app := tview.NewApplication()
	status := tview.NewTextView().
		SetDynamicColors(true).
		SetText("[green]Ready.[white] Type a task and press Enter. Esc or Ctrl+C exits.")

	history := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(true)
	history.SetBorder(true).SetTitle("Enno")
	appendMessage(history, "enno", "Interactive TUI started.")

	input := tview.NewInputField().
		SetLabel("> ").
		SetFieldWidth(0)
	input.SetBorder(true).SetTitle("Prompt")

	root := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(status, 1, 0, false).
		AddItem(history, 0, 1, false).
		AddItem(input, 3, 0, true)

	busy := false
	input.SetDoneFunc(func(key tcell.Key) {
		if key != tcell.KeyEnter {
			return
		}
		if busy {
			return
		}
		query := strings.TrimSpace(input.GetText())
		if query == "" {
			return
		}
		if shouldExit(query) {
			cancel()
			app.Stop()
			return
		}

		busy = true
		input.SetText("")
		status.SetText("[yellow]Running...[white]")
		appendMessage(history, "you", query)

		go func() {
			answer, runErr := agent.Run(ctx, query)
			app.QueueUpdateDraw(func() {
				defer func() {
					busy = false
					status.SetText("[green]Ready.[white] Type another task and press Enter.")
				}()
				if runErr != nil {
					appendMessage(history, "error", runErr.Error())
					return
				}
				if strings.TrimSpace(answer) == "" {
					appendMessage(history, "enno", "(no response)")
					return
				}
				appendMessage(history, "enno", answer)
			})
		}()
	})

	quit := func() {
		cancel()
		app.Stop()
	}
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape, tcell.KeyCtrlC:
			quit()
			return nil
		default:
			return event
		}
	})

	return app.SetRoot(root, true).SetFocus(input).Run()
}

func appendMessage(history *tview.TextView, author, message string) {
	color := "white"
	switch author {
	case "enno":
		color = "green"
	case "you":
		color = "blue"
	case "error":
		color = "red"
	}
	fmt.Fprintf(history, "[%s]%s:[white] %s\n\n", color, author, tview.Escape(message))
	history.ScrollToEnd()
}

func plainREPL(ctx context.Context, agent *enno.Agent, config Config) error {
	reader := bufio.NewReader(config.In)
	interactive := isTerminal(config.In)

	for {
		if interactive {
			fmt.Fprint(config.Out, config.Prompt)
		}
		query, err := reader.ReadString('\n')
		if err != nil && len(query) == 0 {
			if interactive {
				fmt.Fprintln(config.Out)
			}
			if err == io.EOF {
				return nil
			}
			return err
		}

		query = strings.TrimSpace(query)
		if query == "" {
			continue
		}
		if shouldExit(query) {
			return nil
		}

		answer, err := agent.Run(ctx, query)
		if err != nil {
			fmt.Fprintf(config.Err, "Error: %v\n\n", err)
			continue
		}
		if answer != "" {
			fmt.Fprintln(config.Out, answer)
		}
		fmt.Fprintln(config.Out)
	}
}

func shouldExit(query string) bool {
	return strings.EqualFold(query, "q") || strings.EqualFold(query, "exit")
}

func (c Config) withDefaults() Config {
	if c.Prompt == "" {
		c.Prompt = "\033[36menno >> \033[0m"
	}
	if c.In == nil {
		c.In = os.Stdin
	}
	if c.Out == nil {
		c.Out = os.Stdout
	}
	if c.Err == nil {
		c.Err = os.Stderr
	}
	return c
}

func isTerminal(reader io.Reader) bool {
	file, ok := reader.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
