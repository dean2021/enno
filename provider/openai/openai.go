package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dean2021/enno"
	openaisdk "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
)

type Config struct {
	APIKey  string
	BaseURL string
	Model   string
}

type Provider struct {
	client openaisdk.Client
	model  string
}

func New(config Config) *Provider {
	options := []option.RequestOption{}
	if config.APIKey != "" {
		options = append(options, option.WithAPIKey(config.APIKey))
	}
	if config.BaseURL != "" {
		options = append(options, option.WithBaseURL(config.BaseURL))
	}
	return &Provider{
		client: openaisdk.NewClient(options...),
		model:  config.Model,
	}
}

func (p *Provider) Complete(ctx context.Context, req enno.Request) (enno.Response, error) {
	messages := make([]openaisdk.ChatCompletionMessageParamUnion, 0, len(req.Messages)+1)
	if req.SystemPrompt != "" {
		messages = append(messages, openaisdk.SystemMessage(req.SystemPrompt))
	}
	for _, message := range req.Messages {
		messages = append(messages, toOpenAIMessage(message))
	}

	resp, err := p.client.Chat.Completions.New(ctx, openaisdk.ChatCompletionNewParams{
		Model:    p.model,
		Messages: messages,
		Tools:    toOpenAITools(req.Tools),
	})
	if err != nil {
		return enno.Response{}, err
	}
	if len(resp.Choices) == 0 {
		return enno.Response{}, fmt.Errorf("empty choices")
	}

	choice := resp.Choices[0]
	msg := choice.Message
	toolCalls := make([]enno.ToolCall, 0, len(msg.ToolCalls))
	for _, toolCall := range msg.ToolCalls {
		functionCall := toolCall.AsFunction()
		toolCalls = append(toolCalls, enno.ToolCall{
			ID:        functionCall.ID,
			Name:      functionCall.Function.Name,
			Arguments: json.RawMessage(functionCall.Function.Arguments),
		})
	}
	return enno.Response{
		Content:   msg.Content,
		Thinking:  openAIAssistantThinking(msg),
		ToolCalls: toolCalls,
		Usage: enno.Usage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
			TotalTokens:  resp.Usage.TotalTokens,
		},
	}, nil
}

// openAIAssistantThinking extracts optional reasoning text from the assistant message.
// OpenAI Chat Completions does not expose chain-of-thought as a first-class field on
// all models; newer APIs may include extra JSON keys such as "reasoning" or
// "reasoning_content". When absent, this returns empty (same as no visible thinking).
func openAIAssistantThinking(msg openaisdk.ChatCompletionMessage) string {
	raw := msg.RawJSON()
	if raw == "" {
		return ""
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return ""
	}
	for _, key := range []string{"reasoning", "reasoning_content", "reasoning_summary"} {
		part, ok := obj[key]
		if !ok {
			continue
		}
		var s string
		if err := json.Unmarshal(part, &s); err == nil && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func toOpenAIMessage(message enno.Message) openaisdk.ChatCompletionMessageParamUnion {
	switch message.Role {
	case enno.RoleUser:
		return openaisdk.UserMessage(message.Content)
	case enno.RoleAssistant:
		assistant := openaisdk.AssistantMessage(message.Content)
		assistant.OfAssistant.ToolCalls = toOpenAIToolCalls(message.ToolCalls)
		return assistant
	case enno.RoleTool:
		return openaisdk.ToolMessage(message.Content, message.ToolCallID)
	default:
		return openaisdk.UserMessage(message.Content)
	}
}

func toOpenAITools(tools []enno.Tool) []openaisdk.ChatCompletionToolUnionParam {
	result := make([]openaisdk.ChatCompletionToolUnionParam, 0, len(tools))
	for _, tool := range tools {
		result = append(result, openaisdk.ChatCompletionFunctionTool(shared.FunctionDefinitionParam{
			Name:        tool.Name,
			Description: openaisdk.String(tool.Description),
			Parameters: shared.FunctionParameters{
				"type":                 "object",
				"properties":           tool.Properties,
				"required":             tool.Required,
				"additionalProperties": false,
			},
		}))
	}
	return result
}

func toOpenAIToolCalls(toolCalls []enno.ToolCall) []openaisdk.ChatCompletionMessageToolCallUnionParam {
	result := make([]openaisdk.ChatCompletionMessageToolCallUnionParam, 0, len(toolCalls))
	for _, toolCall := range toolCalls {
		result = append(result, openaisdk.ChatCompletionMessageToolCallUnionParam{
			OfFunction: &openaisdk.ChatCompletionMessageFunctionToolCallParam{
				ID: toolCall.ID,
				Function: openaisdk.ChatCompletionMessageFunctionToolCallFunctionParam{
					Arguments: string(toolCall.Arguments),
					Name:      toolCall.Name,
				},
			},
		})
	}
	return result
}
