package enno

import (
	"context"
	"errors"
	"io"
)

type StreamProvider interface {
	Stream(context.Context, Request) (Stream, error)
}

type Stream interface {
	Next(context.Context) (StreamEvent, error)
	Close() error
}

type StreamEventType string

const (
	StreamEventTextDelta     StreamEventType = "text_delta"
	StreamEventThinkingDelta StreamEventType = "thinking_delta"
	StreamEventToolCallDelta StreamEventType = "tool_call_delta"
	StreamEventFinalResponse StreamEventType = "final_response"
	StreamEventUsage         StreamEventType = "usage"
)

type StreamEvent struct {
	Type     StreamEventType
	Text     string
	Thinking string
	ToolCall ToolCall
	Response Response
	Usage    Usage
	Err      error
}

type StreamHandler func(context.Context, StreamEvent)

func (a *Agent) RunStream(ctx context.Context, input string, handler StreamHandler) (RunResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.session.Append(UserMessage(input))
	return a.runSessionLocked(ctx, &a.session, handler)
}

type responseStream struct {
	events []StreamEvent
	index  int
}

func NewResponseStream(response Response) Stream {
	events := make([]StreamEvent, 0, 4)
	if response.Content != "" {
		events = append(events, StreamEvent{Type: StreamEventTextDelta, Text: response.Content})
	}
	if response.Thinking != "" {
		events = append(events, StreamEvent{Type: StreamEventThinkingDelta, Thinking: response.Thinking})
	}
	for _, toolCall := range response.ToolCalls {
		events = append(events, StreamEvent{Type: StreamEventToolCallDelta, ToolCall: cloneToolCall(toolCall)})
	}
	if response.Usage.InputTokens != 0 || response.Usage.OutputTokens != 0 || response.Usage.TotalTokens != 0 || response.Usage.Estimated {
		events = append(events, StreamEvent{Type: StreamEventUsage, Usage: response.Usage})
	}
	events = append(events, StreamEvent{Type: StreamEventFinalResponse, Response: cloneResponse(response)})
	return &responseStream{events: events}
}

func (s *responseStream) Next(context.Context) (StreamEvent, error) {
	if s.index >= len(s.events) {
		return StreamEvent{}, io.EOF
	}
	event := s.events[s.index]
	s.index++
	return event, nil
}

func (s *responseStream) Close() error {
	return nil
}

func ConsumeStream(ctx context.Context, stream Stream, handler StreamHandler) (Response, error) {
	if stream == nil {
		return Response{}, errors.New("enno: nil stream")
	}
	defer stream.Close()

	var response Response
	for {
		event, err := stream.Next(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return response, nil
			}
			return response, err
		}
		if handler != nil {
			handler(ctx, event)
		}
		switch event.Type {
		case StreamEventTextDelta:
			response.Content += event.Text
		case StreamEventThinkingDelta:
			response.Thinking += event.Thinking
		case StreamEventToolCallDelta:
			response.ToolCalls = append(response.ToolCalls, cloneToolCall(event.ToolCall))
		case StreamEventUsage:
			response.Usage = event.Usage
		case StreamEventFinalResponse:
			response = cloneResponse(event.Response)
		}
	}
}
