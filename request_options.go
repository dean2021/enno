package enno

type RequestOptions struct {
	Temperature     *float64
	MaxOutputTokens int64
	ToolChoice      ToolChoice
	ResponseFormat  ResponseFormat
	Metadata        map[string]string
}

type ToolChoice struct {
	Type string
	Name string
}

const (
	ToolChoiceAuto     = "auto"
	ToolChoiceNone     = "none"
	ToolChoiceRequired = "required"
	ToolChoiceTool     = "tool"
)

type ResponseFormat struct {
	Type        string
	Name        string
	Description string
	Schema      map[string]any
	Strict      *bool
}

const (
	ResponseFormatText       = "text"
	ResponseFormatJSONObject = "json_object"
	ResponseFormatJSONSchema = "json_schema"
)

func (o RequestOptions) IsZero() bool {
	return o.Temperature == nil &&
		o.MaxOutputTokens == 0 &&
		o.ToolChoice.Type == "" &&
		o.ResponseFormat.Type == "" &&
		len(o.Metadata) == 0
}

func (o RequestOptions) WithDefaults(defaults RequestOptions) RequestOptions {
	if o.Temperature == nil {
		o.Temperature = defaults.Temperature
	}
	if o.MaxOutputTokens == 0 {
		o.MaxOutputTokens = defaults.MaxOutputTokens
	}
	if o.ToolChoice.Type == "" {
		o.ToolChoice = defaults.ToolChoice
	}
	if o.ResponseFormat.Type == "" {
		o.ResponseFormat = defaults.ResponseFormat
	}
	if len(o.Metadata) == 0 {
		o.Metadata = cloneStringMap(defaults.Metadata)
	} else {
		o.Metadata = cloneStringMap(o.Metadata)
	}
	return o
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}
