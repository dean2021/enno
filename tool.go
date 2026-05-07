package enno

import (
	"context"
	"encoding/json"
	"fmt"
)

type ToolHandler func(context.Context, json.RawMessage) (string, error)

type Tool struct {
	Name        string
	Description string
	Properties  map[string]any
	Required    []string
	Handler     ToolHandler
}

func NewTool(name, description string, properties map[string]any, required []string, handler ToolHandler) Tool {
	return Tool{
		Name:        name,
		Description: description,
		Properties:  properties,
		Required:    required,
		Handler:     handler,
	}
}

func NewTypedTool[T any](name, description string, properties map[string]any, required []string, handler func(context.Context, T) (string, error)) Tool {
	return NewTool(name, description, properties, required, func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args T
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("invalid %s arguments: %w", name, err)
		}
		return handler(ctx, args)
	})
}

func ToolMap(tools []Tool) map[string]Tool {
	result := make(map[string]Tool, len(tools))
	for _, tool := range tools {
		result[tool.Name] = tool
	}
	return result
}
