package cliui

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/dean2021/enno"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type Config struct {
	Prompt string
	In     io.Reader
	Out    io.Writer
	Err    io.Writer
	Events <-chan enno.Event
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

	events := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(true)
	events.SetBorder(true).SetTitle("Run Details")
	fmt.Fprintln(events, "[gray]Waiting for events.[white]")

	input := tview.NewInputField().
		SetLabel("> ").
		SetFieldWidth(0)
	input.SetBorder(true).SetTitle("Prompt")

	body := tview.NewFlex().
		AddItem(history, 0, 2, false).
		AddItem(events, 0, 1, false)

	root := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(status, 1, 0, false).
		AddItem(body, 0, 1, false).
		AddItem(input, 3, 0, true)

	startTUIEventLoop(ctx, app, status, events, config.Events)

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

func startTUIEventLoop(ctx context.Context, app *tview.Application, status, events *tview.TextView, stream <-chan enno.Event) {
	if stream == nil {
		return
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-stream:
				app.QueueUpdateDraw(func() {
					renderEvent(status, events, event)
				})
			}
		}
	}()
}

func renderEvent(status, events *tview.TextView, event enno.Event) {
	switch event.Type {
	case enno.EventModelStart:
		status.SetText(fmt.Sprintf("[yellow]Calling model...[white] round=%d messages=%d tools=%d", event.Round, event.MessageCount, event.ToolCount))
	case enno.EventModelResponse:
		status.SetText(fmt.Sprintf("[green]Model response.[white] %s", formatUsage(event.Usage)))
	case enno.EventToolStart:
		status.SetText(fmt.Sprintf("[yellow]Running tool %s...[white]", event.ToolCall.Name))
	case enno.EventToolResult:
		status.SetText(fmt.Sprintf("[green]Tool %s done.[white] %s", event.ToolCall.Name, event.Duration.Round(time.Millisecond)))
	case enno.EventRoundComplete:
		status.SetText(fmt.Sprintf("[green]Round %d complete.[white] %s", event.Round, formatUsage(event.Usage)))
	case enno.EventError:
		status.SetText(fmt.Sprintf("[red]Error:[white] %s", tview.Escape(errorString(event.Err))))
	}
	fmt.Fprintln(events, formatEvent(event))
	events.ScrollToEnd()
}

func formatEvent(event enno.Event) string {
	switch event.Type {
	case enno.EventModelStart:
		return fmt.Sprintf("[yellow]model_start[white] round=%d messages=%d tools=%d estimated_input=%d",
			event.Round, event.MessageCount, event.ToolCount, event.Usage.InputTokens)
	case enno.EventModelResponse:
		return fmt.Sprintf("[green]model_response[white] round=%d duration=%s %s content=%q",
			event.Round, event.Duration.Round(time.Millisecond), formatUsage(event.Usage), truncate(event.Content, 120))
	case enno.EventToolStart:
		return fmt.Sprintf("[blue]tool_start[white] round=%d name=%s args=%s",
			event.Round, event.ToolCall.Name, truncate(string(event.ToolCall.Arguments), 160))
	case enno.EventToolResult:
		return fmt.Sprintf("[blue]tool_result[white] round=%d name=%s duration=%s result=%s",
			event.Round, event.ToolCall.Name, event.Duration.Round(time.Millisecond), truncate(event.ToolResult, 160))
	case enno.EventRoundComplete:
		return fmt.Sprintf("[green]round_complete[white] round=%d messages=%d %s",
			event.Round, event.MessageCount, formatUsage(event.Usage))
	case enno.EventError:
		return fmt.Sprintf("[red]error[white] %s", tview.Escape(errorString(event.Err)))
	default:
		return fmt.Sprintf("[gray]%s[white]", event.Type)
	}
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
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	startPlainEventLoop(ctx, config.Err, config.Events)

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

func startPlainEventLoop(ctx context.Context, out io.Writer, stream <-chan enno.Event) {
	if out == nil || stream == nil {
		return
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-stream:
				fmt.Fprintf(out, "%s\n", stripColorTags(formatEvent(event)))
			}
		}
	}()
}

func formatUsage(usage enno.Usage) string {
	if usage.InputTokens == 0 && usage.OutputTokens == 0 && usage.TotalTokens == 0 {
		return "usage=unknown"
	}
	quality := "exact"
	if usage.Estimated {
		quality = "estimated"
	}
	return fmt.Sprintf("tokens[%s]=in:%d out:%d total:%d", quality, usage.InputTokens, usage.OutputTokens, usage.TotalTokens)
}

func truncate(value string, limit int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "(empty)"
	}
	if len(value) <= limit {
		return tview.Escape(value)
	}
	return tview.Escape(value[:limit]) + "..."
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func stripColorTags(value string) string {
	replacer := strings.NewReplacer(
		"[yellow]", "",
		"[green]", "",
		"[blue]", "",
		"[red]", "",
		"[gray]", "",
		"[white]", "",
	)
	return replacer.Replace(value)
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
