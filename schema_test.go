package enno

import (
	"context"
	"strings"
	"testing"
)

func TestSchemaObjectBuilder(t *testing.T) {
	schema := SchemaObject().
		StringProp("name").
		IntegerProp("count").
		BooleanProp("active").
		EnumProp("mode", "fast", "safe").
		Required("name", "mode")

	properties := schema.Properties()
	if properties["name"].(map[string]any)["type"] != "string" {
		t.Fatalf("name property = %#v", properties["name"])
	}
	if properties["count"].(map[string]any)["type"] != "integer" {
		t.Fatalf("count property = %#v", properties["count"])
	}
	if properties["active"].(map[string]any)["type"] != "boolean" {
		t.Fatalf("active property = %#v", properties["active"])
	}
	enum := properties["mode"].(map[string]any)["enum"].([]string)
	if len(enum) != 2 || enum[0] != "fast" || enum[1] != "safe" {
		t.Fatalf("mode enum = %#v", enum)
	}
	required := schema.RequiredFields()
	if len(required) != 2 || required[0] != "name" || required[1] != "mode" {
		t.Fatalf("required = %#v", required)
	}

	enum[0] = "mutated"
	if schema.Properties()["mode"].(map[string]any)["enum"].([]string)[0] != "fast" {
		t.Fatal("Properties should return a copy")
	}
}

func TestNewTypedToolFromSchema(t *testing.T) {
	type args struct {
		Name string `json:"name"`
	}
	tool := NewTypedToolFromSchema("greet", "Greet.", SchemaObject().
		StringProp("name").
		Required("name"), func(_ context.Context, args args) (string, error) {
		return "hello " + args.Name, nil
	})

	if tool.Properties["name"].(map[string]any)["type"] != "string" {
		t.Fatalf("tool properties = %#v", tool.Properties)
	}
	if len(tool.Required) != 1 || tool.Required[0] != "name" {
		t.Fatalf("tool required = %#v", tool.Required)
	}
}

func TestNewAgentRejectsDuplicateToolNames(t *testing.T) {
	_, err := NewAgent(Config{
		Provider: staticProvider{resp: Response{Content: "unused"}},
		Tools: []Tool{
			NewTool("dup", "first", nil, nil, nil),
			NewTool("dup", "second", nil, nil, nil),
		},
	})
	if err == nil || !strings.Contains(err.Error(), `duplicate tool name "dup"`) {
		t.Fatalf("NewAgent error = %v", err)
	}
}

func TestNewAgentRejectsInvalidToolName(t *testing.T) {
	_, err := NewAgent(Config{
		Provider: staticProvider{resp: Response{Content: "unused"}},
		Tools: []Tool{
			NewTool("bad name", "bad", nil, nil, nil),
		},
	})
	if err == nil || !strings.Contains(err.Error(), "invalid tool name") {
		t.Fatalf("NewAgent error = %v", err)
	}
}

func TestNewAgentRejectsUnknownRequiredProperty(t *testing.T) {
	_, err := NewAgent(Config{
		Provider: staticProvider{resp: Response{Content: "unused"}},
		Tools: []Tool{
			NewTool("bad", "bad", map[string]any{
				"known": map[string]any{"type": "string"},
			}, []string{"missing"}, nil),
		},
	})
	if err == nil || !strings.Contains(err.Error(), `requires unknown property "missing"`) {
		t.Fatalf("NewAgent error = %v", err)
	}
}
