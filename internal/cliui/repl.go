package cliui

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/dean2021/enno"
	"github.com/dean2021/enno/internal/history"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Idle title for the main transcript pane (also restored after a busy spinner stops).
const mainViewTitleIdle = "Enno"

type Config struct {
	Prompt   string
	In       io.Reader
	Out      io.Writer
	Err      io.Writer
	Events   <-chan enno.Event
	Recorder *history.Recorder
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

	tview.Styles.PrimitiveBackgroundColor = tcell.ColorDefault
	// Softer than default sharp corners + bright white borders.
	tview.Borders.TopLeft = tview.BoxDrawingsLightArcDownAndRight
	tview.Borders.TopRight = tview.BoxDrawingsLightArcDownAndLeft
	tview.Borders.BottomLeft = tview.BoxDrawingsLightArcUpAndRight
	tview.Borders.BottomRight = tview.BoxDrawingsLightArcUpAndLeft
	// Focused primitives default to double-line borders; match unfocused light +
	// rounded corners so the prompt pane matches the main pane.
	tview.Borders.HorizontalFocus = tview.BoxDrawingsLightHorizontal
	tview.Borders.VerticalFocus = tview.BoxDrawingsLightVertical
	tview.Borders.TopLeftFocus = tview.BoxDrawingsLightArcDownAndRight
	tview.Borders.TopRightFocus = tview.BoxDrawingsLightArcDownAndLeft
	tview.Borders.BottomLeftFocus = tview.BoxDrawingsLightArcUpAndRight
	tview.Borders.BottomRightFocus = tview.BoxDrawingsLightArcUpAndLeft

	app := tview.NewApplication()
	status := tview.NewTextView().
		SetDynamicColors(true).
		SetText("[green]Ready.[white] ↑↓ history · wheel scrolls main pane · Shift+drag select (some terminals) · Esc exits.")
	status.SetBackgroundColor(tcell.ColorDefault)

	mainView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(true)
	mainView.SetBackgroundColor(tcell.ColorDefault)
	borderMuted := tcell.StyleDefault.
		Foreground(tcell.ColorGray).
		Background(tcell.ColorDefault)
	titleMuted := tcell.ColorDarkGray
	mainState := newMainViewState()
	mainState.AppendMessage("enno", "Interactive TUI started.")
	mainView.SetBorder(true).
		SetBorderStyle(borderMuted).
		SetTitleColor(titleMuted).
		SetTitleAlign(tview.AlignLeft).
		SetBorderPadding(0, 0, 1, 0).
		SetTitle(mainViewTitleIdle)
	renderMainView(mainView, mainState, true)

	// Load recent history entries for Up/Down navigation in the prompt.
	var histEntries []string
	if config.Recorder != nil {
		entries, err := history.LoadRecent(config.Recorder.Path(), 500)
		if err == nil {
			for _, e := range entries {
				if e.Display != "" {
					histEntries = append(histEntries, e.Display)
				}
			}
		}
	}
	inputHist := newInputHistory(histEntries)

	input := tview.NewInputField().
		SetLabel("> ").
		SetFieldWidth(0).
		SetFieldBackgroundColor(tcell.ColorDefault)
	input.SetBackgroundColor(tcell.ColorDefault)
	input.SetBorder(true).
		SetBorderStyle(borderMuted).
		SetTitleColor(titleMuted).
		SetTitleAlign(tview.AlignLeft).
		SetBorderPadding(0, 0, 1, 0).
		SetTitle("Prompt")

	// Up/Down browse input history. Main transcript scrolls only via mouse wheel over
	// that pane (see SetMouseCapture); keyboard does not scroll the main window.
	input.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyUp:
			if text, ok := inputHist.Up(); ok {
				input.SetText(text)
			}
			return nil
		case tcell.KeyDown:
			if text, ok := inputHist.Down(); ok {
				input.SetText(text)
			}
			return nil
		default:
			inputHist.ResetDraft(input.GetText())
			return event
		}
	})

	root := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(status, 1, 0, false).
		AddItem(mainView, 0, 1, false).
		AddItem(input, 3, 0, true)

	followOutput := true
	startTUIEventLoop(ctx, app, status, mainView, mainState, &followOutput, config.Events)

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
		status.SetText("[yellow]Waiting for model…[white]")
		stopBusySpinner := startBusyMainTitleSpinner(ctx, app, mainView)
		mainState.AppendMessage("you", query)
		followOutput = true
		renderMainView(mainView, mainState, followOutput)

		inputHist.Append(query)

		if config.Recorder != nil {
			_ = config.Recorder.Record(query)
		}

		go func() {
			answer, runErr := agent.Run(ctx, query)
			stopBusySpinner()
			app.QueueUpdateDraw(func() {
				mainView.SetTitle(mainViewTitleIdle)
				defer func() {
					busy = false
					status.SetText("[green]Ready.[white] ↑↓ history · wheel scrolls main pane · Shift+drag select (some terminals) · Esc exits.")
				}()
				if runErr != nil {
					mainState.AppendMessage("error", runErr.Error())
					renderMainView(mainView, mainState, followOutput)
					return
				}
				if strings.TrimSpace(answer) == "" {
					mainState.AppendMessage("enno", "(no response)")
					renderMainView(mainView, mainState, followOutput)
					return
				}
				mainState.AppendMessage("enno", answer)
				renderMainView(mainView, mainState, followOutput)
			})
		}()
	})

	quit := func() {
		cancel()
		app.Stop()
	}

	// tview's default is no mouse. We enable only XTerm "normal tracking" (clicks +
	// wheel as button events, not 1002/1003 motion), then route wheel over the main
	// pane to scroll. Native drag-select still works in many terminals when holding
	// Shift (or disabling mouse capture entirely — then wheel stops working).
	app.EnableMouse(false)
	var initMouse sync.Once
	app.SetBeforeDrawFunc(func(s tcell.Screen) bool {
		initMouse.Do(func() {
			s.EnableMouse(tcell.MouseButtonEvents)
		})
		return false
	})

	app.SetMouseCapture(func(event *tcell.EventMouse, action tview.MouseAction) (*tcell.EventMouse, tview.MouseAction) {
		switch action {
		case tview.MouseScrollUp, tview.MouseScrollDown:
			x, y := event.Position()
			if mainView.InRect(x, y) {
				if action == tview.MouseScrollUp {
					scrollMainTowardOlder(mainView, 3, &followOutput)
				} else {
					scrollMainTowardNewer(mainView, 3, &followOutput)
				}
				return nil, 0
			}
		}
		return event, action
	})

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape, tcell.KeyCtrlC:
			quit()
			return nil
		}
		return event
	})
	return app.SetRoot(root, true).SetFocus(input).Run()
}

// startBusyMainTitleSpinner rotates the main pane title while waiting on the model,
// so long network calls do not feel frozen.
func startBusyMainTitleSpinner(ctx context.Context, app *tview.Application, mainView *tview.TextView) context.CancelFunc {
	spinCtx, cancel := context.WithCancel(ctx)
	go func() {
		frames := []rune("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")
		n := len(frames)
		i := 0
		rotate := func() {
			ch := frames[i%n]
			i++
			app.QueueUpdateDraw(func() {
				mainView.SetTitle(fmt.Sprintf("%s  Enno · waiting for model…", string(ch)))
			})
		}
		rotate()
		tick := time.NewTicker(90 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-spinCtx.Done():
				return
			case <-tick.C:
				rotate()
			}
		}
	}()
	return cancel
}

func startTUIEventLoop(ctx context.Context, app *tview.Application, status, mainView *tview.TextView, mainState *mainViewState, followOutput *bool, stream <-chan enno.Event) {
	if stream == nil {
		return
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-stream:
				if !ok {
					return
				}
				app.QueueUpdateDraw(func() {
					renderEvent(status, mainView, mainState, followOutput, event)
				})
			}
		}
	}()
}

func renderEvent(status, mainView *tview.TextView, mainState *mainViewState, followOutput *bool, event enno.Event) {
	mainState.AppendEvent(event)
	status.SetText(formatStatusLine(event))
	renderMainView(mainView, mainState, *followOutput)
}

// scrollMainTowardOlder moves the main transcript toward older lines (smaller row offset).
func scrollMainTowardOlder(mainView *tview.TextView, lines int, followOutput *bool) {
	row, col := mainView.GetScrollOffset()
	*followOutput = false
	mainView.ScrollTo(max(0, row-lines), col)
}

// scrollMainTowardNewer moves toward newer lines; if already at the end, re-enables follow-latest.
func scrollMainTowardNewer(mainView *tview.TextView, lines int, followOutput *bool) {
	row, col := mainView.GetScrollOffset()
	mainView.ScrollTo(row+lines, col)
	newRow, _ := mainView.GetScrollOffset()
	if newRow == row {
		*followOutput = true
	}
}

func renderMainView(mainView *tview.TextView, mainState *mainViewState, followOutput bool) {
	mainView.SetText(mainState.Render())
	scrollToLatestIfFollowing(mainView, followOutput)
}

func scrollToLatestIfFollowing(mainView *tview.TextView, followOutput bool) {
	if followOutput {
		forceScrollToLatest(mainView)
	}
}

func forceScrollToLatest(mainView *tview.TextView) {
	// Only ScrollToEnd: ScrollTo(row, col) sets trackEnd=false in tview and would
	// break auto-follow on the next append.
	mainView.ScrollToEnd()
}

type mainViewState struct {
	Messages []chatMessage
}

type chatMessage struct {
	Author  string
	Message string
	Rich    bool
}

func newMainViewState() *mainViewState {
	return &mainViewState{}
}

func (s *mainViewState) AppendMessage(author, message string) {
	s.Messages = append(s.Messages, chatMessage{Author: author, Message: message})
}

func (s *mainViewState) AppendRichMessage(author, message string) {
	s.Messages = append(s.Messages, chatMessage{Author: author, Message: message, Rich: true})
}

func (s *mainViewState) AppendEvent(event enno.Event) {
	if message := formatEventMessage(event); message != "" {
		s.AppendRichMessage(eventAuthor(event), message)
	}
}

func (s *mainViewState) Render() string {
	var builder strings.Builder
	for _, message := range s.Messages {
		content := tview.Escape(message.Message)
		if message.Rich {
			content = message.Message
		}
		if message.Author == "" {
			fmt.Fprintf(&builder, "%s\n\n", content)
			continue
		}
		fmt.Fprintf(&builder, "[%s]%s:[white] %s\n\n", authorColor(message.Author), tview.Escape(message.Author), content)
	}
	return builder.String()
}

func authorColor(author string) string {
	switch author {
	case "enno":
		return "green"
	case "you":
		return "blue"
	case "error":
		return "red"
	case "tool":
		// Distinct from formatToolInvocation's aqua tool name so "tool:" does not blend into bash(...).
		return "gray"
	default:
		return "white"
	}
}

func eventAuthor(event enno.Event) string {
	switch event.Type {
	case enno.EventToolStart:
		return "tool"
	case enno.EventToolResult:
		return ""
	case enno.EventError:
		return "error"
	default:
		return "enno"
	}
}

func formatEventMessage(event enno.Event) string {
	switch event.Type {
	case enno.EventModelStart:
		return ""
	case enno.EventModelResponse:
		if strings.TrimSpace(event.Thinking) == "" {
			return ""
		}
		return fmt.Sprintf("[yellow]Thinking[white]: %s", tview.Escape(summarize(event.Thinking, 220)))
	case enno.EventToolStart:
		return formatToolInvocation(event.ToolCall, 160)
	case enno.EventToolResult:
		return fmt.Sprintf("[white]Result: %s[white]", tview.Escape(summarize(event.ToolResult, 180)))
	case enno.EventRoundComplete:
		return ""
	case enno.EventError:
		return fmt.Sprintf("[red]Error[white]: %s", tview.Escape(errorString(event.Err)))
	default:
		return ""
	}
}

func formatEvent(event enno.Event) string {
	switch event.Type {
	case enno.EventModelStart:
		return fmt.Sprintf("[yellow]Round %d: calling model[white] with %d messages and %d tools. %s",
			event.Round, event.MessageCount, event.ToolCount, formatUsage(event.Usage))
	case enno.EventModelResponse:
		return fmt.Sprintf("[green]Round %d: model responded[white] in %s. %s visible_output=%q",
			event.Round, event.Duration.Round(time.Millisecond), formatUsage(event.Usage), summarize(event.Content, 120))
	case enno.EventToolStart:
		return fmt.Sprintf("%s started in round %d", formatToolInvocation(event.ToolCall, 160), event.Round)
	case enno.EventToolResult:
		return fmt.Sprintf("[white]Result[white]: %s", summarize(event.ToolResult, 160))
	case enno.EventRoundComplete:
		return fmt.Sprintf("[green]Round %d complete[white] with %d messages. %s",
			event.Round, event.MessageCount, formatUsage(event.Usage))
	case enno.EventError:
		return fmt.Sprintf("[red]Error[white]: %s", tview.Escape(errorString(event.Err)))
	default:
		return fmt.Sprintf("[gray]%s[white]", event.Type)
	}
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

		if config.Recorder != nil {
			_ = config.Recorder.Record(query)
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
			case event, ok := <-stream:
				if !ok {
					return
				}
				fmt.Fprintf(out, "%s\n", stripColorTags(formatEvent(event)))
			}
		}
	}()
}

func formatStatusLine(event enno.Event) string {
	switch event.Type {
	case enno.EventModelStart:
		return fmt.Sprintf("[yellow]Calling model...[white] round=%d messages=%d tools=%d", event.Round, event.MessageCount, event.ToolCount)
	case enno.EventModelResponse:
		return fmt.Sprintf("[green]Model responded.[white] %s", formatUsage(event.Usage))
	case enno.EventToolStart:
		return fmt.Sprintf("[yellow]Running tool[white] [aqua]%s[white]", tview.Escape(event.ToolCall.Name))
	case enno.EventToolResult:
		return fmt.Sprintf("[green]Tool complete[white] [aqua]%s[white] %s", tview.Escape(event.ToolCall.Name), event.Duration.Round(time.Millisecond))
	case enno.EventRoundComplete:
		return fmt.Sprintf("[green]Round %d complete.[white] %s", event.Round, formatUsage(event.Usage))
	case enno.EventError:
		return fmt.Sprintf("[red]Error:[white] %s", tview.Escape(errorString(event.Err)))
	default:
		return "[green]Ready.[white]"
	}
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

func formatToolInvocation(toolCall enno.ToolCall, limit int) string {
	argument := summarizeToolArgument(toolCall.Arguments, limit)
	if argument == "" {
		return fmt.Sprintf("[aqua]%s[white]()", tview.Escape(toolCall.Name))
	}
	return fmt.Sprintf("[aqua]%s[white]([purple]%s[white])", tview.Escape(toolCall.Name), tview.Escape(argument))
}

func summarizeToolArgument(raw json.RawMessage, limit int) string {
	if len(raw) == 0 {
		return ""
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return summarize(string(raw), limit)
	}
	if object, ok := value.(map[string]any); ok {
		for _, key := range []string{"command", "path", "query", "prompt"} {
			if text, ok := object[key].(string); ok {
				return quoteJSONString(summarize(text, limit))
			}
		}
		if len(object) == 1 {
			for _, rawValue := range object {
				if text, ok := rawValue.(string); ok {
					return quoteJSONString(summarize(text, limit))
				}
			}
		}
	}
	return summarizeJSON(raw, limit)
}

func quoteJSONString(value string) string {
	bytes, err := json.Marshal(value)
	if err != nil {
		return value
	}
	return string(bytes)
}

func summarizeJSON(raw json.RawMessage, limit int) string {
	if len(raw) == 0 {
		return "(empty)"
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return summarize(string(raw), limit)
	}
	compact, err := json.Marshal(value)
	if err != nil {
		return summarize(string(raw), limit)
	}
	return summarize(string(compact), limit)
}

func summarize(value string, limit int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "(empty)"
	}
	value = strings.Join(strings.Fields(value), " ")
	if len(value) <= limit {
		return value
	}
	if limit <= 3 {
		return value[:limit]
	}
	return value[:limit-3] + "..."
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
		"[aqua]", "",
		"[purple]", "",
		"[red]", "",
		"[gray]", "",
		"[white]", "",
	)
	return replacer.Replace(value)
}

// inputHistory manages navigation through previously entered inputs via Up/Down keys.
type inputHistory struct {
	entries []string // chronological order (oldest first)
	index   int      // current position: len(entries) means "at the draft"
	draft   string   // text the user was typing before navigating up
}

func newInputHistory(entries []string) *inputHistory {
	return &inputHistory{
		entries: entries,
		index:   len(entries),
	}
}

// Up moves to an older entry. Returns the text to display and true if the position changed.
func (h *inputHistory) Up() (string, bool) {
	if h.index <= 0 {
		return "", false
	}
	h.index--
	return h.entries[h.index], true
}

// Down moves to a newer entry. Returns the text to display and true if the position changed.
func (h *inputHistory) Down() (string, bool) {
	if h.index >= len(h.entries) {
		return "", false
	}
	h.index++
	if h.index >= len(h.entries) {
		return h.draft, true
	}
	return h.entries[h.index], true
}

// ResetDraft saves the current input text as the draft and resets navigation to the bottom.
func (h *inputHistory) ResetDraft(text string) {
	h.draft = text
	h.index = len(h.entries)
}

// Append adds a new entry to the history and resets navigation.
func (h *inputHistory) Append(text string) {
	h.entries = append(h.entries, text)
	h.index = len(h.entries)
	h.draft = ""
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
