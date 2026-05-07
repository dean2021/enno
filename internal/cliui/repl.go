package cliui

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dean2021/enno"
	"github.com/marcusolsson/tui-go"
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

	status := tui.NewLabel("Ready. Type a task and press Enter. Esc or Ctrl+C exits.")
	status.SetSizePolicy(tui.Expanding, tui.Maximum)

	history := tui.NewVBox()
	appendMessage(history, "enno", "Interactive TUI started.")

	historyScroll := tui.NewScrollArea(history)
	historyScroll.SetAutoscrollToBottom(true)
	historyBox := tui.NewVBox(historyScroll)
	historyBox.SetBorder(true)

	input := tui.NewEntry()
	input.SetFocused(true)
	input.SetSizePolicy(tui.Expanding, tui.Maximum)

	inputBox := tui.NewHBox(input)
	inputBox.SetBorder(true)
	inputBox.SetSizePolicy(tui.Expanding, tui.Maximum)

	root := tui.NewVBox(status, historyBox, inputBox)
	root.SetSizePolicy(tui.Expanding, tui.Expanding)

	ui, err := tui.New(root)
	if err != nil {
		return err
	}

	busy := false
	input.OnSubmit(func(entry *tui.Entry) {
		if busy {
			return
		}
		query := strings.TrimSpace(entry.Text())
		if query == "" {
			return
		}
		if shouldExit(query) {
			cancel()
			ui.Quit()
			return
		}

		busy = true
		entry.SetText("")
		status.SetText("Running...")
		appendMessage(history, "you", query)

		go func() {
			answer, runErr := agent.Run(ctx, query)
			ui.Update(func() {
				defer func() {
					busy = false
					status.SetText("Ready. Type another task and press Enter.")
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
		ui.Quit()
	}
	ui.SetKeybinding("Esc", quit)
	ui.SetKeybinding("Ctrl+C", quit)
	focusChain := &tui.SimpleFocusChain{}
	focusChain.Set(input)
	ui.SetFocusChain(focusChain)

	return ui.Run()
}

func appendMessage(history *tui.Box, author, message string) {
	label := tui.NewLabel(fmt.Sprintf("%s: %s", author, message))
	label.SetWordWrap(true)
	label.SetSizePolicy(tui.Expanding, tui.Maximum)
	history.Append(label)
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
