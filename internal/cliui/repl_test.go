package cliui

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/dean2021/enno"
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
		Thinking: "I should inspect the repository before answering.",
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

	rendered := stripANSI(state.ViewportString(80, -1))
	for _, want := range []string{
		"You",
		"run tests",
		"Thinking",
		"I should inspect the repository before answering.",
		"bash",
		"ls -la",
		"Result",
		"file content",
		"Enno",
		"running",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected rendered content to contain %q, got:\n%s", want, rendered)
		}
	}
	for _, notWant := range []string{"Model responded", "Round 4 complete", "tokens=in:8756"} {
		if strings.Contains(rendered, notWant) {
			t.Fatalf("did not expect %q in rendered output, got:\n%s", notWant, rendered)
		}
	}
}

func TestModelStartDoesNotRenderFakeThinking(t *testing.T) {
	state := newMainViewState()
	state.AppendEvent(enno.Event{
		Type:         enno.EventModelStart,
		Round:        8,
		MessageCount: 32,
		ToolCount:    5,
		Usage:        enno.Usage{InputTokens: 8999, Estimated: true},
	})

	rendered := stripANSI(state.ViewportString(80, -1))
	if strings.Contains(rendered, "Thinking...") || strings.Contains(rendered, "round=8") {
		t.Fatalf("did not expect fake thinking status in rendered output, got:\n%s", rendered)
	}
}

func TestModelResponseRendersExplicitThinkingOnly(t *testing.T) {
	withoutAnything := formatEventMessage(enno.Event{Type: enno.EventModelResponse})
	if withoutAnything != "" {
		t.Fatalf("expected no message without thinking or duration, got %q", withoutAnything)
	}

	withThinking := formatEventMessage(enno.Event{
		Type:     enno.EventModelResponse,
		Thinking: "visible reasoning summary",
	})
	if !strings.Contains(withThinking, "Thinking") || !strings.Contains(withThinking, "visible reasoning summary") {
		t.Fatalf("unexpected thinking message: %q", withThinking)
	}

	withDuration := formatEventMessage(enno.Event{
		Type:     enno.EventModelResponse,
		Duration: 3 * time.Second,
	})
	if !strings.Contains(withDuration, "\u23F1") {
		t.Fatalf("expected duration in message, got %q", withDuration)
	}

	withBoth := formatEventMessage(enno.Event{
		Type:     enno.EventModelResponse,
		Thinking: "reasoning",
		Duration: 5 * time.Second,
	})
	if !strings.Contains(withBoth, "Thinking") || !strings.Contains(withBoth, "\u23F1") {
		t.Fatalf("expected both thinking and duration, got %q", withBoth)
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
	if got != "[white]\u25B8 Result: ok[white]" {
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

func TestToolResultStoresFullContent(t *testing.T) {
	state := newMainViewState()
	state.AppendEvent(enno.Event{
		Type:       enno.EventToolResult,
		ToolCall:   enno.ToolCall{Name: "bash"},
		ToolResult: strings.Repeat("line\n", 10),
		Duration:   10 * time.Millisecond,
	})
	if len(state.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(state.Messages))
	}
	msg := state.Messages[0]
	if msg.FullContent == "" {
		t.Fatal("expected FullContent to be set for tool result")
	}
	if strings.Count(msg.FullContent, "\n") != 10 {
		t.Fatalf("expected 10 newlines in FullContent, got %d", strings.Count(msg.FullContent, "\n"))
	}
	if msg.Expanded {
		t.Fatal("expected Expanded to be false by default")
	}
}

func TestToggleExpandAtLine(t *testing.T) {
	state := newMainViewState()
	state.AppendMessage("enno", "hello")
	state.AppendEvent(enno.Event{
		Type:       enno.EventToolResult,
		ToolCall:   enno.ToolCall{Name: "bash"},
		ToolResult: strings.Repeat("output\n", 5),
		Duration:   10 * time.Millisecond,
	})
	state.AppendMessage("enno", "done")

	_ = state.ViewportString(80, -1)

	if len(state.msgStartLines) < 3 {
		t.Fatalf("expected at least 3 msgStartLines, got %d", len(state.msgStartLines))
	}

	toolResultIdx := -1
	for i, msg := range state.Messages {
		if msg.FullContent != "" {
			toolResultIdx = i
			break
		}
	}
	if toolResultIdx < 0 {
		t.Fatal("no expandable message found")
	}

	targetLine := state.msgStartLines[toolResultIdx]
	if !state.ToggleExpandAtLine(targetLine) {
		t.Fatal("expected ToggleExpandAtLine to find and toggle expandable message")
	}
	if !state.Messages[toolResultIdx].Expanded {
		t.Fatal("expected message to be expanded")
	}

	state.ToggleExpandAtLine(targetLine)
	if state.Messages[toolResultIdx].Expanded {
		t.Fatal("expected message to be collapsed after second toggle")
	}

	gapLine := state.msgStartLines[toolResultIdx+1] - 1
	if state.MessageIndexAtLine(gapLine) != -1 {
		t.Fatal("separator line should not map to a message")
	}
	if state.ToggleExpandAtLine(gapLine) {
		t.Fatal("separator line should not toggle the previous expandable message")
	}

	if state.ToggleExpandAtLine(0) {
		t.Fatal("line 0 points to non-expandable message, should not toggle")
	}
}

func TestTranscriptContentLineMapsScreenRowsToViewportContent(t *testing.T) {
	m := &bubbleModel{vp: viewport.New(80, 5)}
	m.vp.YOffset = 10

	if _, ok := m.transcriptContentLine(0); ok {
		t.Fatal("top border should not map to content")
	}
	if got, ok := m.transcriptContentLine(1); !ok || got != 10 {
		t.Fatalf("first content row = %d, %v; want 10, true", got, ok)
	}
	if got, ok := m.transcriptContentLine(5); !ok || got != 14 {
		t.Fatalf("last content row = %d, %v; want 14, true", got, ok)
	}
	if _, ok := m.transcriptContentLine(6); ok {
		t.Fatal("bottom border should not map to content")
	}
}

func TestUpdateHoverUsesTranscriptContentRows(t *testing.T) {
	state := newMainViewState()
	state.AppendEvent(enno.Event{
		Type:       enno.EventToolResult,
		ToolCall:   enno.ToolCall{Name: "bash"},
		ToolResult: strings.Repeat("output\n", 5),
		Duration:   10 * time.Millisecond,
	})
	state.AppendMessage("enno", "done")

	m := &bubbleModel{
		width:     80,
		vp:        viewport.New(80, 5),
		mainState: state,
		hoverLine: -1,
	}
	m.syncViewport()

	m.updateHover(1)
	if m.hoverLine != 0 {
		t.Fatalf("hover over first content row selected line %d, want 0", m.hoverLine)
	}
	m.updateHover(0)
	if m.hoverLine != -1 {
		t.Fatalf("hover over top border selected line %d, want -1", m.hoverLine)
	}
	m.updateHover(6)
	if m.hoverLine != -1 {
		t.Fatalf("hover over bottom border selected line %d, want -1", m.hoverLine)
	}
}

func TestHoverHighlightDoesNotChangeLineMap(t *testing.T) {
	state := newMainViewState()
	state.Messages = append(state.Messages,
		chatMessage{
			Author:      "tool",
			Message:     strings.Repeat("x", 120),
			FullContent: "expanded output",
		},
		chatMessage{Author: "enno", Message: "after"},
	)

	unhovered := state.ViewportString(40, -1)
	unhoveredStarts := append([]int(nil), state.msgStartLines...)
	unhoveredEnds := append([]int(nil), state.msgEndLines...)
	unhoveredLines := append([]renderLine(nil), state.renderLines...)
	hovered := state.ViewportString(40, 0)
	hoveredStarts := append([]int(nil), state.msgStartLines...)
	hoveredEnds := append([]int(nil), state.msgEndLines...)
	hoveredLines := append([]renderLine(nil), state.renderLines...)

	if strings.Count(unhovered, "\n") != strings.Count(hovered, "\n") {
		t.Fatalf("hover changed rendered line count:\nunhovered:\n%s\nhovered:\n%s", unhovered, hovered)
	}
	if len(unhoveredStarts) != len(hoveredStarts) {
		t.Fatalf("line map length changed from %d to %d", len(unhoveredStarts), len(hoveredStarts))
	}
	for i := range unhoveredStarts {
		if unhoveredStarts[i] != hoveredStarts[i] {
			t.Fatalf("line map changed at %d: unhovered=%v hovered=%v", i, unhoveredStarts, hoveredStarts)
		}
	}
	if len(unhoveredEnds) != len(hoveredEnds) {
		t.Fatalf("line map end length changed from %d to %d", len(unhoveredEnds), len(hoveredEnds))
	}
	for i := range unhoveredEnds {
		if unhoveredEnds[i] != hoveredEnds[i] {
			t.Fatalf("line map ends changed at %d: unhovered=%v hovered=%v", i, unhoveredEnds, hoveredEnds)
		}
	}
	if len(unhoveredLines) != len(hoveredLines) {
		t.Fatalf("render line count changed from %d to %d", len(unhoveredLines), len(hoveredLines))
	}
	for i := range unhoveredLines {
		if unhoveredLines[i] != hoveredLines[i] {
			t.Fatalf("render line metadata changed at %d: unhovered=%#v hovered=%#v", i, unhoveredLines[i], hoveredLines[i])
		}
	}
}

func TestViewportStringLineMapUsesWrappedRows(t *testing.T) {
	state := newMainViewState()
	state.Messages = append(state.Messages,
		chatMessage{
			Author:      "tool",
			Message:     strings.Repeat("x", 90),
			FullContent: "expanded output",
		},
		chatMessage{Author: "enno", Message: "after"},
	)

	rendered := state.ViewportString(30, -1)
	if state.msgEndLines[0] <= state.msgStartLines[0] {
		t.Fatalf("expected first message to wrap across multiple rows, line map=%v..%v\n%s", state.msgStartLines, state.msgEndLines, rendered)
	}
	wrappedRow := state.msgStartLines[0] + 1
	if got := state.MessageIndexAtLine(wrappedRow); got != 0 {
		t.Fatalf("wrapped row mapped to message %d, want 0; line map=%v..%v", got, state.msgStartLines, state.msgEndLines)
	}
	if line := state.renderLines[wrappedRow]; line.MessageIndex != 0 || !line.Expandable {
		t.Fatalf("wrapped row render metadata = %#v, want message 0 expandable", line)
	}
	if state.msgStartLines[1] <= state.msgEndLines[0] {
		t.Fatalf("second message starts before first wrapped message ends: starts=%v ends=%v", state.msgStartLines, state.msgEndLines)
	}
}

func TestSyncViewportRecomputesHoverAfterFollowOutputAppend(t *testing.T) {
	state := newMainViewState()
	for i := 0; i < 8; i++ {
		state.AppendMessage("enno", strings.Repeat("old", 20))
	}
	m := &bubbleModel{
		width:        40,
		vp:           viewport.New(40, 4),
		mainState:    state,
		followOutput: true,
		hoverLine:    -1,
		lastMouseY:   1,
		hasMouseY:    true,
	}
	m.syncViewport()
	firstHoverLine := m.hoverLine
	if firstHoverLine != m.vp.YOffset {
		t.Fatalf("hover line should track first visible content row: hover=%d yoffset=%d", firstHoverLine, m.vp.YOffset)
	}

	for i := 0; i < 8; i++ {
		state.AppendMessage("enno", strings.Repeat("new", 20))
	}
	m.syncViewport()
	if m.hoverLine != m.vp.YOffset {
		t.Fatalf("hover line was not recomputed after append: hover=%d yoffset=%d previous=%d", m.hoverLine, m.vp.YOffset, firstHoverLine)
	}
	if m.hoverLine == firstHoverLine {
		t.Fatalf("hover line stayed on stale content after follow-output append: %d", m.hoverLine)
	}
}

func TestModelResponseShowsDuration(t *testing.T) {
	event := enno.Event{
		Type:     enno.EventModelResponse,
		Duration: 3 * time.Second,
	}
	msg := formatEventMessage(event)
	if !strings.Contains(msg, "3s") {
		t.Fatalf("expected duration in message, got %q", msg)
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
