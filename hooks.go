package enno

import (
	"context"
	"fmt"
)

type Hook interface {
	BeforeProviderCall(context.Context, BeforeProviderCallState) (BeforeProviderCallResult, error)
	AfterProviderCall(context.Context, AfterProviderCallState) (AfterProviderCallResult, error)
	BeforeToolCall(context.Context, BeforeToolCallState) (BeforeToolCallResult, error)
	AfterToolCall(context.Context, AfterToolCallState) (AfterToolCallResult, error)
}

type NoopHook struct{}

func (NoopHook) BeforeProviderCall(context.Context, BeforeProviderCallState) (BeforeProviderCallResult, error) {
	return BeforeProviderCallResult{}, nil
}

func (NoopHook) AfterProviderCall(context.Context, AfterProviderCallState) (AfterProviderCallResult, error) {
	return AfterProviderCallResult{}, nil
}

func (NoopHook) BeforeToolCall(context.Context, BeforeToolCallState) (BeforeToolCallResult, error) {
	return BeforeToolCallResult{}, nil
}

func (NoopHook) AfterToolCall(context.Context, AfterToolCallState) (AfterToolCallResult, error) {
	return AfterToolCallResult{}, nil
}

type BeforeProviderCallState struct {
	Round   int
	Request Request
}

type BeforeProviderCallResult struct {
	Request *Request
	Abort   error
}

type AfterProviderCallState struct {
	Round    int
	Request  Request
	Response Response
}

type AfterProviderCallResult struct {
	Response *Response
	Abort    error
}

type BeforeToolCallState struct {
	Round    int
	Request  Request
	ToolCall ToolCall
}

type BeforeToolCallResult struct {
	ToolCall    *ToolCall
	Deny        bool
	DenyMessage string
	Abort       error
}

type AfterToolCallState struct {
	Round      int
	Request    Request
	ToolCall   ToolCall
	ToolResult ToolResult
	Err        error
}

type AfterToolCallResult struct {
	ToolResult *ToolResult
	Abort      error
}

func (a *Agent) beforeProviderCallHooks(ctx context.Context, round int, request Request) (Request, error) {
	for _, hook := range a.hooks {
		result, err := hook.BeforeProviderCall(ctx, BeforeProviderCallState{
			Round:   round,
			Request: cloneRequest(request),
		})
		if err != nil {
			return Request{}, err
		}
		if result.Abort != nil {
			return Request{}, result.Abort
		}
		if result.Request != nil {
			request = cloneRequest(*result.Request)
		}
	}
	return request, nil
}

func (a *Agent) afterProviderCallHooks(ctx context.Context, round int, request Request, response Response) (Response, error) {
	for _, hook := range a.hooks {
		result, err := hook.AfterProviderCall(ctx, AfterProviderCallState{
			Round:    round,
			Request:  cloneRequest(request),
			Response: cloneResponse(response),
		})
		if err != nil {
			return Response{}, err
		}
		if result.Abort != nil {
			return Response{}, result.Abort
		}
		if result.Response != nil {
			response = cloneResponse(*result.Response)
		}
	}
	return response, nil
}

func (a *Agent) beforeToolCallHooks(ctx context.Context, round int, request Request, toolCall ToolCall) (ToolCall, *ToolResult, error) {
	for _, hook := range a.hooks {
		result, err := hook.BeforeToolCall(ctx, BeforeToolCallState{
			Round:    round,
			Request:  cloneRequest(request),
			ToolCall: cloneToolCall(toolCall),
		})
		if err != nil {
			return ToolCall{}, nil, err
		}
		if result.Abort != nil {
			return ToolCall{}, nil, result.Abort
		}
		if result.ToolCall != nil {
			toolCall = cloneToolCall(*result.ToolCall)
		}
		if result.Deny {
			message := result.DenyMessage
			if message == "" {
				message = fmt.Sprintf("Error: tool %s denied by hook", toolCall.Name)
			}
			toolResult := ToolResult{
				Content: message,
				Error:   true,
			}
			return toolCall, &toolResult, nil
		}
	}
	return toolCall, nil, nil
}

func (a *Agent) afterToolCallHooks(ctx context.Context, round int, request Request, toolCall ToolCall, toolResult ToolResult, toolErr error) (ToolResult, error) {
	for _, hook := range a.hooks {
		result, err := hook.AfterToolCall(ctx, AfterToolCallState{
			Round:      round,
			Request:    cloneRequest(request),
			ToolCall:   cloneToolCall(toolCall),
			ToolResult: toolResult,
			Err:        toolErr,
		})
		if err != nil {
			return ToolResult{}, err
		}
		if result.Abort != nil {
			return ToolResult{}, result.Abort
		}
		if result.ToolResult != nil {
			toolResult = normalizeToolResult(*result.ToolResult)
		}
	}
	return toolResult, nil
}

func cloneRequest(request Request) Request {
	request.Messages = cloneMessages(request.Messages)
	request.Tools = append([]Tool(nil), request.Tools...)
	request.Options = request.Options.WithDefaults(RequestOptions{})
	return request
}

func cloneResponse(response Response) Response {
	response.ToolCalls = cloneToolCalls(response.ToolCalls)
	return response
}
