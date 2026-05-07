package cliui

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/dean2021/enno"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type stubProvider struct {
	calls int
}

func (p *stubProvider) Complete(_ context.Context, req enno.Request) (enno.Response, error) {
	p.calls++
	last := req.Messages[len(req.Messages)-1]
	return enno.Response{Content: "answer: " + last.Content}, nil
}

func TestREPLPlainFallbackRunsPrompt(t *testing.T) {
	provider := &stubProvider{}
	agent, err := enno.NewAgent(enno.Config{Provider: provider})
	if err != nil {
		t.Fatalf("new agent: %v", err)
	}

	var out strings.Builder
	err = REPL(context.Background(), agent, Config{
		In:  strings.NewReader("hello\nexit\n"),
		Out: &out,
	})
	if err != nil {
		t.Fatalf("repl: %v", err)
	}

	if provider.calls != 1 {
		t.Fatalf("expected 1 provider call, got %d", provider.calls)
	}
	if !strings.Contains(out.String(), "answer: hello") {
		t.Fatalf("expected answer in output, got %q", out.String())
	}
}

func TestREPLPlainFallbackSkipsEmptyInputAndExits(t *testing.T) {
	provider := &stubProvider{}
	agent, err := enno.NewAgent(enno.Config{Provider: provider})
	if err != nil {
		t.Fatalf("new agent: %v", err)
	}

	err = REPL(context.Background(), agent, Config{
		In: strings.NewReader("\nq\n"),
	})
	if err != nil {
		t.Fatalf("repl: %v", err)
	}
	if provider.calls != 0 {
		t.Fatalf("expected no provider calls, got %d", provider.calls)
	}
}

func TestMainViewStateAppendsEventsToConversation(t *testing.T) {
	state := newMainViewState()
	state.AppendMessage("you", "run tests")
	state.AppendEvent(enno.Event{
		Type:         enno.EventModelStart,
		Round:        1,
		MessageCount: 4,
		ToolCount:    2,
		Usage:        enno.Usage{InputTokens: 120, Estimated: true},
	})
	state.AppendEvent(enno.Event{
		Type:     enno.EventModelResponse,
		Round:    1,
		Usage:    enno.Usage{InputTokens: 8756, OutputTokens: 867, TotalTokens: 9623},
		Duration: 36 * time.Second,
	})
	state.AppendEvent(enno.Event{
		Type:     enno.EventToolStart,
		Round:    1,
		ToolCall: enno.ToolCall{Name: "bash", Arguments: json.RawMessage(`{"command":"ls -la /Users/deanlu/Desktop/sources/my_projects/enno"}`)},
	})
	state.AppendEvent(enno.Event{
		Type:       enno.EventToolResult,
		Round:      1,
		ToolCall:   enno.ToolCall{Name: "bash"},
		ToolResult: strings.Repeat("file content ", 40),
		Duration:   42 * time.Millisecond,
	})
	state.AppendEvent(enno.Event{
		Type:  enno.EventRoundComplete,
		Round: 4,
		Usage: enno.Usage{InputTokens: 8756, OutputTokens: 867, TotalTokens: 9623},
	})
	state.AppendMessage("enno", "running")

	rendered := state.Render()
	for _, want := range []string{
		"[blue]you:[white] run tests",
		"[green]enno:[white] [yellow]Thinking...",
		"[aqua]tool:[white] [aqua]bash[white]([purple]\"ls -la /Users/deanlu/Desktop/sources/my_projects/enno\"[white])",
		"[white]Result: file content",
		"[green]enno:[white] running",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected main view to contain %q, got:\n%s", want, rendered)
		}
	}
	for _, notWant := range []string{"Status\n", "Current", "Context", "Timeline", "Conversation", "Model responded", "Round 4 complete", "tokens=in:8756", "Tool:", "Params:", "completed in", "tool: Result"} {
		if strings.Contains(rendered, notWant) {
			t.Fatalf("did not expect panel heading %q in conversation stream, got:\n%s", notWant, rendered)
		}
	}
}

func TestToolResultMessageOmitsCompletionLine(t *testing.T) {
	got := formatEventMessage(enno.Event{
		Type:       enno.EventToolResult,
		ToolCall:   enno.ToolCall{Name: "bash"},
		ToolResult: "ok",
		Duration:   10 * time.Millisecond,
	})

	if strings.Contains(got, "completed in") || strings.Contains(got, "bash") {
		t.Fatalf("expected compact result without completion line, got %q", got)
	}
	if got != "[white]Result: ok[white]" {
		t.Fatalf("unexpected tool result message: %q", got)
	}
}

func TestFormatEventUsesHumanReadableSentences(t *testing.T) {
	event := enno.Event{
		Type:     enno.EventToolStart,
		Round:    2,
		ToolCall: enno.ToolCall{Name: "bash", Arguments: json.RawMessage(`{"command":"go test ./..."}`)},
	}

	got := stripColorTags(formatEvent(event))
	if !strings.Contains(got, `bash("go test ./...") started in round 2`) {
		t.Fatalf("expected human readable tool sentence, got %q", got)
	}
	if strings.Contains(got, "tool_start") {
		t.Fatalf("did not expect raw event name, got %q", got)
	}
	colored := formatEvent(event)
	if !strings.Contains(colored, `[aqua]bash[white]([purple]"go test ./..."[white])`) {
		t.Fatalf("expected colored tool invocation, got %q", colored)
	}
}

func TestFormatToolInvocationUsesPrimaryArgument(t *testing.T) {
	got := formatToolInvocation(enno.ToolCall{
		Name:      "read_file",
		Arguments: json.RawMessage(`{"path":"/tmp/example.txt"}`),
	}, 80)

	if got != `[aqua]read_file[white]([purple]"/tmp/example.txt"[white])` {
		t.Fatalf("unexpected invocation format: %q", got)
	}
}

func TestSummarizeJSONCompactsAndTruncates(t *testing.T) {
	got := summarizeJSON(json.RawMessage(`{
		"command": "go test ./internal/cliui",
		"reason": "verify run details state rendering"
	}`), 32)

	if len(got) > 32 {
		t.Fatalf("expected truncated summary length <= 32, got %d: %q", len(got), got)
	}
	if !strings.Contains(got, `"command"`) {
		t.Fatalf("expected compact JSON summary, got %q", got)
	}
}

func TestHandleMainViewScrollKeys(t *testing.T) {
	view := tview.NewTextView().SetScrollable(true)
	view.ScrollTo(20, 0)
	followOutput := true

	if !handleMainViewScroll(tcell.NewEventKey(tcell.KeyPgUp, 0, tcell.ModNone), view, &followOutput) {
		t.Fatal("expected PageUp to be handled")
	}
	row, _ := view.GetScrollOffset()
	if followOutput {
		t.Fatal("expected PageUp to disable following latest output")
	}
	if row != 10 {
		t.Fatalf("expected PageUp to scroll to row 10, got %d", row)
	}

	if !handleMainViewScroll(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone), view, &followOutput) {
		t.Fatal("expected End to be handled")
	}
	if !followOutput {
		t.Fatal("expected End to follow latest output again")
	}
}

func TestRenderMainViewForcesLatestWhenFollowing(t *testing.T) {
	view := tview.NewTextView().SetScrollable(true)
	state := newMainViewState()
	for range 50 {
		state.AppendMessage("enno", "line")
	}

	renderMainView(view, state, true)
	row, _ := view.GetScrollOffset()
	if row <= 0 {
		t.Fatalf("expected following render to move scroll offset toward the end, got %d", row)
	}

	handleMainViewScroll(tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModNone), view, new(bool))
	renderMainView(view, state, false)
	row, _ = view.GetScrollOffset()
	if row != 0 {
		t.Fatalf("expected non-following render to preserve manual scroll, got %d", row)
	}
}
