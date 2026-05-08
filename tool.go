package enno

import (
	"context"
	"encoding/json"
	"fmt"
)

type ToolHandler func(context.Context, json.RawMessage) (string, error)

type StructuredToolHandler func(context.Context, json.RawMessage) (ToolResult, error)

type ToolResult struct {
	Content  string
	Error    bool
	Metadata map[string]any
}

type Tool struct {
	Name              string
	Description       string
	Properties        map[string]any
	Required          []string
	Handler           ToolHandler
	StructuredHandler StructuredToolHandler
}

func NewTool(name, description string, properties map[string]any, required []string, handler ToolHandler) Tool {
	return Tool{
		Name:              name,
		Description:       description,
		Properties:        properties,
		Required:          required,
		Handler:           handler,
		StructuredHandler: wrapToolHandler(handler),
	}
}

func NewStructuredTool(name, description string, properties map[string]any, required []string, handler StructuredToolHandler) Tool {
	return Tool{
		Name:              name,
		Description:       description,
		Properties:        properties,
		Required:          required,
		StructuredHandler: handler,
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			if handler == nil {
				return "", nil
			}
			result, err := handler(ctx, raw)
			return result.Content, err
		},
	}
}

func NewTypedToolFromSchema[T any](name, description string, schema *Schema, handler func(context.Context, T) (string, error)) Tool {
	if schema == nil {
		return NewTypedTool(name, description, nil, nil, handler)
	}
	return NewTypedTool(name, description, schema.Properties(), schema.RequiredFields(), handler)
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

func validateTools(tools []Tool) error {
	seen := make(map[string]struct{}, len(tools))
	for _, tool := range tools {
		if err := validateToolDefinition(tool); err != nil {
			return err
		}
		if _, ok := seen[tool.Name]; ok {
			return fmt.Errorf("enno: duplicate tool name %q", tool.Name)
		}
		seen[tool.Name] = struct{}{}
	}
	return nil
}

func validateToolDefinition(tool Tool) error {
	if err := validateToolName(tool.Name); err != nil {
		return err
	}
	required := make(map[string]struct{}, len(tool.Required))
	for _, name := range tool.Required {
		if name == "" {
			return fmt.Errorf("enno: tool %q has an empty required property", tool.Name)
		}
		if _, ok := required[name]; ok {
			return fmt.Errorf("enno: tool %q has duplicate required property %q", tool.Name, name)
		}
		required[name] = struct{}{}
		if _, ok := tool.Properties[name]; !ok {
			return fmt.Errorf("enno: tool %q requires unknown property %q", tool.Name, name)
		}
	}
	return nil
}

func validateToolName(name string) error {
	if name == "" {
		return fmt.Errorf("enno: tool name cannot be empty")
	}
	if len(name) > 64 {
		return fmt.Errorf("enno: invalid tool name %q: must be 64 characters or fewer", name)
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return fmt.Errorf("enno: invalid tool name %q: use only letters, numbers, underscores, or hyphens", name)
	}
	return nil
}

func wrapToolHandler(handler ToolHandler) StructuredToolHandler {
	if handler == nil {
		return nil
	}
	return func(ctx context.Context, raw json.RawMessage) (ToolResult, error) {
		output, err := handler(ctx, raw)
		return ToolResult{Content: output}, err
	}
}
