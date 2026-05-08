package cliui

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/dean2021/enno"
	"github.com/dean2021/enno/internal/history"
)

// Idle title for the main transcript pane (also restored after a busy spinner stops).
const mainViewTitleIdle = "Enno"

// Shown when idle (status bar); keep in sync with busy-handler resets.
const tuiReadyHint = "[green]Ready.[white] Tab transcript for wheel · Alt+↑↓ prompt history · Ctrl+↑↓ scroll from prompt · / Ctrl+F search · gg/G · Esc→prompt / quit · [gray]copy: focus prompt, or Shift+drag when on transcript[white]"

type Config struct {
	Prompt   string
	In       io.Reader
	Out      io.Writer
	Err      io.Writer
	Events   <-chan enno.Event
	Recorder *history.Recorder
	// DisableMouse matches Claude Code CLAUDE_CODE_DISABLE_MOUSE: skip terminal mouse capture; alternate screen unchanged.
	DisableMouse bool
}

func REPL(ctx context.Context, agent *enno.Agent, config Config) error {
	config = config.withDefaults()
	if !isTerminal(config.In) {
		return plainREPL(ctx, agent, config)
	}
	return bubbleteaREPL(ctx, agent, config)
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
		content := escapeTagLike(message.Message)
		if message.Rich {
			content = message.Message
		}
		if message.Author == "" {
			fmt.Fprintf(&builder, "%s\n\n", content)
			continue
		}
		fmt.Fprintf(&builder, "[%s]%s:[white] %s\n\n", authorColor(message.Author), DisplayAuthor(message.Author), content)
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
		return fmt.Sprintf("[yellow]Thinking[white]: %s", escapeTagLike(summarize(event.Thinking, 220)))
	case enno.EventToolStart:
		return formatToolInvocation(event.ToolCall, 160)
	case enno.EventToolResult:
		return fmt.Sprintf("[white]Result: %s[white]", escapeTagLike(summarize(event.ToolResult, 180)))
	case enno.EventRoundComplete:
		return ""
	case enno.EventError:
		return fmt.Sprintf("[red]Error[white]: %s", escapeTagLike(errorString(event.Err)))
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
		return fmt.Sprintf("[red]Error[white]: %s", escapeTagLike(errorString(event.Err)))
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
		return fmt.Sprintf("[yellow]Running tool[white] [aqua]%s[white]", escapeTagLike(event.ToolCall.Name))
	case enno.EventToolResult:
		return fmt.Sprintf("[green]Tool complete[white] [aqua]%s[white] %s", escapeTagLike(event.ToolCall.Name), event.Duration.Round(time.Millisecond))
	case enno.EventRoundComplete:
		return fmt.Sprintf("[green]Round %d complete.[white] %s", event.Round, formatUsage(event.Usage))
	case enno.EventError:
		return fmt.Sprintf("[red]Error:[white] %s", escapeTagLike(errorString(event.Err)))
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
		return fmt.Sprintf("[aqua]%s[white]()", escapeTagLike(toolCall.Name))
	}
	return fmt.Sprintf("[aqua]%s[white]([purple]%s[white])", escapeTagLike(toolCall.Name), escapeTagLike(argument))
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
