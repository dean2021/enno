package loadskill

import (
	"context"
	"errors"
	"strings"

	"github.com/dean2021/enno"
)

const defaultToolName = "load_skill"

// ToolDescription is shown to the model.
const ToolDescription = `Load the full text of a named skill (on-demand domain instructions). Call only when you need to follow that workflow; skill summaries are already listed in the system prompt.`

// NewTool builds the load_skill tool backed by r. Returns an error if r is nil or has no skills.
func NewTool(r *Registry) (enno.Tool, error) {
	if r == nil || r.Count() == 0 {
		return enno.Tool{}, errors.New("loadskill: registry is empty")
	}
	return enno.NewTypedTool(defaultToolName, ToolDescription, map[string]any{
		"name": map[string]any{
			"type":        "string",
			"description": "Skill identifier from the Skills available list.",
		},
	}, []string{"name"}, func(_ context.Context, args struct {
		Name string `json:"name"`
	}) (string, error) {
		return r.GetContent(strings.TrimSpace(args.Name)), nil
	}), nil
}
