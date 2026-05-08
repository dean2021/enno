package enno

import "testing"

func TestRequestOptionsWithDefaults(t *testing.T) {
	temp := 0.3
	defaultTemp := 0.7
	defaults := RequestOptions{
		Temperature:     &defaultTemp,
		MaxOutputTokens: 100,
		ToolChoice:      ToolChoice{Type: ToolChoiceAuto},
		ResponseFormat:  ResponseFormat{Type: ResponseFormatJSONObject},
		Metadata:        map[string]string{"trace": "default"},
	}

	options := RequestOptions{
		Temperature: &temp,
		Metadata:    map[string]string{"trace": "override"},
	}.WithDefaults(defaults)

	if options.Temperature == nil || *options.Temperature != temp {
		t.Fatalf("temperature = %#v", options.Temperature)
	}
	if options.MaxOutputTokens != 100 {
		t.Fatalf("max output tokens = %d", options.MaxOutputTokens)
	}
	if options.ToolChoice.Type != ToolChoiceAuto {
		t.Fatalf("tool choice = %#v", options.ToolChoice)
	}
	if options.ResponseFormat.Type != ResponseFormatJSONObject {
		t.Fatalf("response format = %#v", options.ResponseFormat)
	}
	if options.Metadata["trace"] != "override" {
		t.Fatalf("metadata = %#v", options.Metadata)
	}
	options.Metadata["trace"] = "mutated"
	if defaults.Metadata["trace"] != "default" {
		t.Fatalf("defaults metadata should not be mutated: %#v", defaults.Metadata)
	}
}
