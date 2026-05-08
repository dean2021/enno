package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	anthropicsdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	anthropicparam "github.com/anthropics/anthropic-sdk-go/packages/param"
	"github.com/dean2021/enno"
	"github.com/dean2021/enno/internal/httpproxy"
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
	// HTTPProxy is an optional proxy URL (http(s):// or socks5://). Empty uses SDK default (environment proxy vars still apply).
	HTTPProxy string
}

type Provider struct {
	client    anthropicsdk.Client
	model     string
	maxTokens int64
}

func New(config Config) (*Provider, error) {
	options := []option.RequestOption{}
	if config.APIKey != "" {
		options = append(options, option.WithAPIKey(config.APIKey))
	}
	maxRetries := defaultHTTPMaxRetries
	if config.MaxHTTPRetries > 0 {
		maxRetries = config.MaxHTTPRetries
	}
	options = append(options, option.WithMaxRetries(maxRetries))
	if strings.TrimSpace(config.HTTPProxy) != "" {
		h, err := httpproxy.Client(config.HTTPProxy)
		if err != nil {
			return nil, err
		}
		if h != nil {
			options = append(options, option.WithHTTPClient(h))
		}
	}
	return &Provider{
		client:    anthropicsdk.NewClient(options...),
		model:     config.Model,
		maxTokens: config.MaxTokens,
	}, nil
}

func (p *Provider) Complete(ctx context.Context, req enno.Request) (enno.Response, error) {
	params, err := p.newParams(req)
	if err != nil {
		return enno.Response{}, err
	}
	resp, err := p.client.Messages.New(ctx, params)
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

func (p *Provider) newParams(req enno.Request) (anthropicsdk.MessageNewParams, error) {
	maxTokens := p.maxTokens
	if req.Options.MaxOutputTokens > 0 {
		maxTokens = req.Options.MaxOutputTokens
	}
	params := anthropicsdk.MessageNewParams{
		Model:     anthropicsdk.Model(p.model),
		MaxTokens: maxTokens,
		System:    toAnthropicSystem(req.SystemPrompt),
		Messages:  toAnthropicMessages(req.Messages),
		Tools:     toAnthropicTools(req.Tools),
	}
	if err := applyAnthropicOptions(&params, req.Options); err != nil {
		return anthropicsdk.MessageNewParams{}, err
	}
	return params, nil
}

func applyAnthropicOptions(params *anthropicsdk.MessageNewParams, options enno.RequestOptions) error {
	if options.Temperature != nil {
		params.Temperature = param.NewOpt(*options.Temperature)
	}
	toolChoice, err := toAnthropicToolChoice(options.ToolChoice)
	if err != nil {
		return err
	}
	params.ToolChoice = toolChoice
	if len(options.Metadata) > 0 {
		userID, ok := options.Metadata["user_id"]
		if !ok || len(options.Metadata) > 1 {
			return fmt.Errorf("%w: anthropic only supports metadata user_id", enno.ErrUnsupportedOption)
		}
		params.Metadata = anthropicsdk.MetadataParam{UserID: param.NewOpt(userID)}
	}
	outputConfig, err := toAnthropicOutputConfig(options.ResponseFormat)
	if err != nil {
		return err
	}
	params.OutputConfig = outputConfig
	return nil
}

func toAnthropicToolChoice(choice enno.ToolChoice) (anthropicsdk.ToolChoiceUnionParam, error) {
	switch choice.Type {
	case "":
		return anthropicsdk.ToolChoiceUnionParam{}, nil
	case enno.ToolChoiceAuto:
		return anthropicsdk.ToolChoiceUnionParam{OfAuto: &anthropicsdk.ToolChoiceAutoParam{}}, nil
	case enno.ToolChoiceNone:
		return anthropicsdk.ToolChoiceUnionParam{OfNone: &anthropicsdk.ToolChoiceNoneParam{}}, nil
	case enno.ToolChoiceRequired:
		return anthropicsdk.ToolChoiceUnionParam{OfAny: &anthropicsdk.ToolChoiceAnyParam{}}, nil
	case enno.ToolChoiceTool:
		if choice.Name == "" {
			return anthropicsdk.ToolChoiceUnionParam{}, fmt.Errorf("%w: tool choice requires name", enno.ErrUnsupportedOption)
		}
		return anthropicsdk.ToolChoiceUnionParam{OfTool: &anthropicsdk.ToolChoiceToolParam{Name: choice.Name}}, nil
	default:
		return anthropicsdk.ToolChoiceUnionParam{}, fmt.Errorf("%w: unsupported tool choice %q", enno.ErrUnsupportedOption, choice.Type)
	}
}

func toAnthropicOutputConfig(format enno.ResponseFormat) (anthropicsdk.OutputConfigParam, error) {
	switch format.Type {
	case "":
		return anthropicsdk.OutputConfigParam{}, nil
	case enno.ResponseFormatJSONSchema:
		if len(format.Schema) == 0 {
			return anthropicsdk.OutputConfigParam{}, fmt.Errorf("%w: json_schema response format requires schema", enno.ErrUnsupportedOption)
		}
		return anthropicsdk.OutputConfigParam{
			Format: anthropicsdk.JSONOutputFormatParam{Schema: cloneMapAny(format.Schema)},
		}, nil
	case enno.ResponseFormatText, enno.ResponseFormatJSONObject:
		return anthropicsdk.OutputConfigParam{}, fmt.Errorf("%w: anthropic does not support response format %q", enno.ErrUnsupportedOption, format.Type)
	default:
		return anthropicsdk.OutputConfigParam{}, fmt.Errorf("%w: unsupported response format %q", enno.ErrUnsupportedOption, format.Type)
	}
}

func cloneMapAny(values map[string]any) map[string]any {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
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
