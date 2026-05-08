package openai

import (
	"strings"
	"testing"

	"github.com/dean2021/enno"
)

func TestApplyOpenAIOptions(t *testing.T) {
	temp := 0.25
	strict := true
	options := enno.RequestOptions{
		Temperature:     &temp,
		MaxOutputTokens: 321,
		ToolChoice:      enno.ToolChoice{Type: enno.ToolChoiceTool, Name: "search"},
		ResponseFormat: enno.ResponseFormat{
			Type:   enno.ResponseFormatJSONSchema,
			Name:   "result",
			Schema: map[string]any{"type": "object"},
			Strict: &strict,
		},
		Metadata: map[string]string{"trace_id": "abc"},
	}

	params, err := (&Provider{model: "gpt-test"}).newParams(enno.Request{
		Options: options,
	})
	if err != nil {
		t.Fatalf("newParams: %v", err)
	}
	if !params.Temperature.Valid() || params.Temperature.Value != temp {
		t.Fatalf("temperature = %#v", params.Temperature)
	}
	if !params.MaxCompletionTokens.Valid() || params.MaxCompletionTokens.Value != 321 {
		t.Fatalf("max completion tokens = %#v", params.MaxCompletionTokens)
	}
	if got := params.ToolChoice.GetFunction(); got == nil || got.Name != "search" {
		t.Fatalf("tool choice = %#v", params.ToolChoice)
	}
	if params.ResponseFormat.OfJSONSchema == nil {
		t.Fatalf("json schema = %#v", params.ResponseFormat)
	}
	if params.Metadata["trace_id"] != "abc" {
		t.Fatalf("metadata = %#v", params.Metadata)
	}
}

func TestOpenAIUnsupportedResponseFormat(t *testing.T) {
	_, err := (&Provider{model: "gpt-test"}).newParams(enno.Request{
		Options: enno.RequestOptions{
			ResponseFormat: enno.ResponseFormat{Type: "xml"},
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), enno.ErrUnsupportedOption.Error()) {
		t.Fatalf("error = %v", err)
	}
}
