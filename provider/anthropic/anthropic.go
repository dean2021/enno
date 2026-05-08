package anthropic

import (
	"context"
	"encoding/json"
	"strings"

	anthropicsdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	anthropicparam "github.com/anthropics/anthropic-sdk-go/packages/param"
	"github.com/dean2021/enno"
)

// defaultHTTPMaxRetries matches provider/openai: Anthropic SDK uses the same Stainless retry policy
// (429, 5xx, timeouts, connection errors, Retry-After). SDK default is 2; we raise the budget for API instability.
const defaultHTTPMaxRetries = 6

type Config struct {
	APIKey    string
	Model     string
	MaxTokens int64
	// MaxHTTPRetries overrides the SDK HTTP retry count when positive (option.WithMaxRetries).
	// Zero selects defaultHTTPMaxRetries.
	MaxHTTPRetries int
}

type Provider struct {
	client    anthropicsdk.Client
	model     string
	maxTokens int64
}

func New(config Config) *Provider {
	options := []option.RequestOption{}
	if config.APIKey != "" {
		options = append(options, option.WithAPIKey(config.APIKey))
	}
	maxRetries := defaultHTTPMaxRetries
	if config.MaxHTTPRetries > 0 {
		maxRetries = config.MaxHTTPRetries
	}
	options = append(options, option.WithMaxRetries(maxRetries))
	return &Provider{
		client:    anthropicsdk.NewClient(options...),
		model:     config.Model,
		maxTokens: config.MaxTokens,
	}
}

func (p *Provider) Complete(ctx context.Context, req enno.Request) (enno.Response, error) {
	resp, err := p.client.Messages.New(ctx, anthropicsdk.MessageNewParams{
		Model:     anthropicsdk.Model(p.model),
		MaxTokens: p.maxTokens,
		System:    toAnthropicSystem(req.SystemPrompt),
		Messages:  toAnthropicMessages(req.Messages),
		Tools:     toAnthropicTools(req.Tools),
	})
	if err != nil {
		return enno.Response{}, err
	}

	var textBlocks []string
	var thinkingBlocks []string
	toolCalls := make([]enno.ToolCall, 0)
	for _, block := range resp.Content {
		switch variant := block.AsAny().(type) {
		case anthropicsdk.TextBlock:
			if variant.Text != "" {
				textBlocks = append(textBlocks, variant.Text)
			}
		case anthropicsdk.ThinkingBlock:
			if variant.Thinking != "" {
				thinkingBlocks = append(thinkingBlocks, variant.Thinking)
			}
		case anthropicsdk.ToolUseBlock:
			toolCalls = append(toolCalls, enno.ToolCall{
				ID:        variant.ID,
				Name:      variant.Name,
				Arguments: variant.Input,
			})
		}
	}
	return enno.Response{
		Content:   strings.Join(textBlocks, "\n"),
		Thinking:  strings.Join(thinkingBlocks, "\n"),
		ToolCalls: toolCalls,
		Usage: enno.Usage{
			InputTokens:  resp.Usage.InputTokens,
			OutputTokens: resp.Usage.OutputTokens,
			TotalTokens:  resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
	}, nil
}

func toAnthropicSystem(systemPrompt string) []anthropicsdk.TextBlockParam {
	if systemPrompt == "" {
		return nil
	}
	return []anthropicsdk.TextBlockParam{{Text: systemPrompt}}
}

func toAnthropicMessages(history []enno.Message) []anthropicsdk.MessageParam {
	messages := make([]anthropicsdk.MessageParam, 0, len(history))
	for i := 0; i < len(history); i++ {
		message := history[i]
		switch message.Role {
		case enno.RoleUser:
			messages = append(messages, anthropicsdk.NewUserMessage(anthropicsdk.NewTextBlock(message.Content)))
		case enno.RoleAssistant:
			messages = append(messages, anthropicsdk.NewAssistantMessage(toAnthropicAssistantBlocks(message)...))
		case enno.RoleTool:
			blocks := []anthropicsdk.ContentBlockParamUnion{}
			for i < len(history) && history[i].Role == enno.RoleTool {
				blocks = append(blocks, anthropicsdk.NewToolResultBlock(history[i].ToolCallID, history[i].Content, false))
				i++
			}
			i--
			messages = append(messages, anthropicsdk.NewUserMessage(blocks...))
		}
	}
	return messages
}

func toAnthropicAssistantBlocks(message enno.Message) []anthropicsdk.ContentBlockParamUnion {
	blocks := make([]anthropicsdk.ContentBlockParamUnion, 0, len(message.ToolCalls)+1)
	if message.Content != "" {
		blocks = append(blocks, anthropicsdk.NewTextBlock(message.Content))
	}
	for _, toolCall := range message.ToolCalls {
		blocks = append(blocks, anthropicsdk.NewToolUseBlock(toolCall.ID, toAnthropicToolInput(toolCall.Arguments), toolCall.Name))
	}
	if len(blocks) == 0 {
		blocks = append(blocks, anthropicsdk.NewTextBlock(""))
	}
	return blocks
}

func toAnthropicToolInput(arguments json.RawMessage) any {
	if len(arguments) == 0 {
		return map[string]any{}
	}
	return arguments
}

func toAnthropicTools(tools []enno.Tool) []anthropicsdk.ToolUnionParam {
	result := make([]anthropicsdk.ToolUnionParam, 0, len(tools))
	for _, tool := range tools {
		converted := anthropicsdk.ToolUnionParamOfTool(anthropicsdk.ToolInputSchemaParam{
			Properties: tool.Properties,
			Required:   tool.Required,
			ExtraFields: map[string]any{
				"additionalProperties": false,
			},
		}, tool.Name)
		converted.OfTool.Description = anthropicparam.NewOpt(tool.Description)
		result = append(result, converted)
	}
	return result
}
