package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/dean2021/enno"
	"github.com/dean2021/enno/provider/internal/httpproxy"
	openaisdk "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/packages/param"
	"github.com/openai/openai-go/v3/shared"
)

// defaultHTTPMaxRetries is the OpenAI SDK [option.WithMaxRetries] value (retries after the first attempt).
// The SDK already retries transient failures: 429, 5xx, request timeout, connection errors, and honors
// Retry-After. Its default is 2 (3 attempts total); we use a higher budget for flaky OpenAI-compatible
// gateways, similar in spirit to Claude Code’s API withRetry (multi-attempt with backoff).
const defaultHTTPMaxRetries = 6

type Config struct {
	APIKey  string
	BaseURL string
	Model   string
	// MaxHTTPRetries overrides the SDK HTTP retry count when positive (option.WithMaxRetries).
	// Zero selects defaultHTTPMaxRetries. The SDK retries 429, 5xx, timeouts, and connection errors.
	MaxHTTPRetries int
	// HTTPProxy is an optional proxy URL (http(s):// or socks5://). Empty uses SDK default (environment proxy vars still apply).
	HTTPProxy string
}

type Provider struct {
	client openaisdk.Client
	model  string
}

func New(config Config) (*Provider, error) {
	options := []option.RequestOption{}
	if config.APIKey != "" {
		options = append(options, option.WithAPIKey(config.APIKey))
	}
	if config.BaseURL != "" {
		options = append(options, option.WithBaseURL(config.BaseURL))
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
		client: openaisdk.NewClient(options...),
		model:  config.Model,
	}, nil
}

func (p *Provider) Complete(ctx context.Context, req enno.Request) (enno.Response, error) {
	params, err := p.newParams(req)
	if err != nil {
		return enno.Response{}, err
	}
	resp, err := p.client.Chat.Completions.New(ctx, params)
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

func (p *Provider) Stream(ctx context.Context, req enno.Request) (enno.Stream, error) {
	params, err := p.newParams(req)
	if err != nil {
		return nil, err
	}
	return &chatCompletionStream{
		stream: p.client.Chat.Completions.NewStreaming(ctx, params),
	}, nil
}

func (p *Provider) newParams(req enno.Request) (openaisdk.ChatCompletionNewParams, error) {
	messages := make([]openaisdk.ChatCompletionMessageParamUnion, 0, len(req.Messages)+1)
	if req.SystemPrompt != "" {
		messages = append(messages, openaisdk.SystemMessage(req.SystemPrompt))
	}
	for _, message := range req.Messages {
		messages = append(messages, toOpenAIMessage(message))
	}

	params := openaisdk.ChatCompletionNewParams{
		Model:    p.model,
		Messages: messages,
		Tools:    toOpenAITools(req.Tools),
	}
	if err := applyOpenAIOptions(&params, req.Options); err != nil {
		return openaisdk.ChatCompletionNewParams{}, err
	}
	return params, nil
}

func applyOpenAIOptions(params *openaisdk.ChatCompletionNewParams, options enno.RequestOptions) error {
	if options.Temperature != nil {
		params.Temperature = param.NewOpt(*options.Temperature)
	}
	if options.MaxOutputTokens > 0 {
		params.MaxCompletionTokens = param.NewOpt(options.MaxOutputTokens)
	}
	params.Metadata = shared.Metadata(cloneMetadata(options.Metadata))
	toolChoice, err := toOpenAIToolChoice(options.ToolChoice)
	if err != nil {
		return err
	}
	params.ToolChoice = toolChoice
	responseFormat, err := toOpenAIResponseFormat(options.ResponseFormat)
	if err != nil {
		return err
	}
	params.ResponseFormat = responseFormat
	return nil
}

func toOpenAIToolChoice(choice enno.ToolChoice) (openaisdk.ChatCompletionToolChoiceOptionUnionParam, error) {
	switch choice.Type {
	case "":
		return openaisdk.ChatCompletionToolChoiceOptionUnionParam{}, nil
	case enno.ToolChoiceAuto, enno.ToolChoiceNone, enno.ToolChoiceRequired:
		return openaisdk.ChatCompletionToolChoiceOptionUnionParam{
			OfAuto: param.NewOpt(choice.Type),
		}, nil
	case enno.ToolChoiceTool:
		if choice.Name == "" {
			return openaisdk.ChatCompletionToolChoiceOptionUnionParam{}, fmt.Errorf("%w: tool choice requires name", enno.ErrUnsupportedOption)
		}
		return openaisdk.ToolChoiceOptionFunctionToolChoice(openaisdk.ChatCompletionNamedToolChoiceFunctionParam{
			Name: choice.Name,
		}), nil
	default:
		return openaisdk.ChatCompletionToolChoiceOptionUnionParam{}, fmt.Errorf("%w: unsupported tool choice %q", enno.ErrUnsupportedOption, choice.Type)
	}
}

func toOpenAIResponseFormat(format enno.ResponseFormat) (openaisdk.ChatCompletionNewParamsResponseFormatUnion, error) {
	switch format.Type {
	case "":
		return openaisdk.ChatCompletionNewParamsResponseFormatUnion{}, nil
	case enno.ResponseFormatText:
		return openaisdk.ChatCompletionNewParamsResponseFormatUnion{
			OfText: &shared.ResponseFormatTextParam{},
		}, nil
	case enno.ResponseFormatJSONObject:
		return openaisdk.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONObject: &shared.ResponseFormatJSONObjectParam{},
		}, nil
	case enno.ResponseFormatJSONSchema:
		if format.Name == "" || len(format.Schema) == 0 {
			return openaisdk.ChatCompletionNewParamsResponseFormatUnion{}, fmt.Errorf("%w: json_schema response format requires name and schema", enno.ErrUnsupportedOption)
		}
		jsonSchema := shared.ResponseFormatJSONSchemaJSONSchemaParam{
			Name:        format.Name,
			Description: param.NewOpt(format.Description),
			Schema:      cloneMetadataAny(format.Schema),
		}
		if format.Strict != nil {
			jsonSchema.Strict = param.NewOpt(*format.Strict)
		}
		return openaisdk.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &shared.ResponseFormatJSONSchemaParam{
				JSONSchema: jsonSchema,
			},
		}, nil
	default:
		return openaisdk.ChatCompletionNewParamsResponseFormatUnion{}, fmt.Errorf("%w: unsupported response format %q", enno.ErrUnsupportedOption, format.Type)
	}
}

func cloneMetadata(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func cloneMetadataAny(values map[string]any) map[string]any {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
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

type chatCompletionStream struct {
	stream    openAIChatChunkStream
	final     enno.Response
	toolCalls map[int64]enno.ToolCall
	finished  bool
}

type openAIChatChunkStream interface {
	Next() bool
	Current() openaisdk.ChatCompletionChunk
	Err() error
	Close() error
}

func (s *chatCompletionStream) Next(context.Context) (enno.StreamEvent, error) {
	if s.finished {
		return enno.StreamEvent{}, io.EOF
	}
	for s.stream.Next() {
		chunk := s.stream.Current()
		if chunk.Usage.PromptTokens != 0 || chunk.Usage.CompletionTokens != 0 || chunk.Usage.TotalTokens != 0 {
			usage := enno.Usage{
				InputTokens:  chunk.Usage.PromptTokens,
				OutputTokens: chunk.Usage.CompletionTokens,
				TotalTokens:  chunk.Usage.TotalTokens,
			}
			s.final.Usage = usage
			return enno.StreamEvent{Type: enno.StreamEventUsage, Usage: usage}, nil
		}
		for _, choice := range chunk.Choices {
			delta := choice.Delta
			if delta.Content != "" {
				s.final.Content += delta.Content
				return enno.StreamEvent{Type: enno.StreamEventTextDelta, Text: delta.Content}, nil
			}
			for _, toolCall := range delta.ToolCalls {
				if s.toolCalls == nil {
					s.toolCalls = map[int64]enno.ToolCall{}
				}
				call := s.toolCalls[toolCall.Index]
				if toolCall.ID != "" {
					call.ID = toolCall.ID
				}
				if toolCall.Function.Name != "" {
					call.Name = toolCall.Function.Name
				}
				if toolCall.Function.Arguments != "" {
					call.Arguments = append(call.Arguments, []byte(toolCall.Function.Arguments)...)
				}
				s.toolCalls[toolCall.Index] = call
				if call.ID != "" || call.Name != "" || len(call.Arguments) > 0 {
					return enno.StreamEvent{Type: enno.StreamEventToolCallDelta, ToolCall: call}, nil
				}
			}
		}
	}
	if err := s.stream.Err(); err != nil {
		return enno.StreamEvent{}, err
	}
	for i := int64(0); i < int64(len(s.toolCalls)); i++ {
		if call, ok := s.toolCalls[i]; ok {
			s.final.ToolCalls = append(s.final.ToolCalls, call)
		}
	}
	s.finished = true
	return enno.StreamEvent{Type: enno.StreamEventFinalResponse, Response: s.final}, nil
}

func (s *chatCompletionStream) Close() error {
	return s.stream.Close()
}
