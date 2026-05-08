package anthropic

import (
	"strings"
	"testing"

	"github.com/dean2021/enno"
)

func TestAnthropicOptionsMapping(t *testing.T) {
	temp := 0.4
	params, err := (&Provider{
		model:     "claude-test",
		maxTokens: 111,
	}).newParams(enno.Request{
		Options: enno.RequestOptions{
			Temperature:     &temp,
			MaxOutputTokens: 222,
			ToolChoice:      enno.ToolChoice{Type: enno.ToolChoiceRequired},
			ResponseFormat: enno.ResponseFormat{
				Type:   enno.ResponseFormatJSONSchema,
				Schema: map[string]any{"type": "object"},
			},
			Metadata: map[string]string{"user_id": "u-1"},
		},
	})
	if err != nil {
		t.Fatalf("newParams: %v", err)
	}
	if params.MaxTokens != 222 {
		t.Fatalf("max tokens = %d, want 222", params.MaxTokens)
	}
	if !params.Temperature.Valid() || params.Temperature.Value != temp {
		t.Fatalf("temperature = %#v", params.Temperature)
	}
	if params.ToolChoice.OfAny == nil {
		t.Fatalf("tool choice = %#v", params.ToolChoice)
	}
	if !params.Metadata.UserID.Valid() || params.Metadata.UserID.Value != "u-1" {
		t.Fatalf("metadata = %#v", params.Metadata)
	}
	if len(params.OutputConfig.Format.Schema) == 0 {
		t.Fatalf("output config = %#v", params.OutputConfig)
	}
}

func TestAnthropicUnsupportedMetadata(t *testing.T) {
	_, err := (&Provider{model: "claude-test", maxTokens: 100}).newParams(enno.Request{
		Options: enno.RequestOptions{
			Metadata: map[string]string{"trace_id": "abc"},
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), enno.ErrUnsupportedOption.Error()) {
		t.Fatalf("error = %v", err)
	}
}
