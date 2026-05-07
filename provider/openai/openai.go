package openai

import (
	"context"
	"encoding/json"
	"fmt"

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
	toolCalls := make([]enno.ToolCall, 0, len(choice.Message.ToolCalls))
	for _, toolCall := range choice.Message.ToolCalls {
		functionCall := toolCall.AsFunction()
		toolCalls = append(toolCalls, enno.ToolCall{
			ID:        functionCall.ID,
			Name:      functionCall.Function.Name,
			Arguments: json.RawMessage(functionCall.Function.Arguments),
		})
	}
	return enno.Response{Content: choice.Message.Content, ToolCalls: toolCalls}, nil
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
