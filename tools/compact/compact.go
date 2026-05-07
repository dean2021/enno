package compact

import (
	"context"

	"github.com/dean2021/enno"
)

// ToolDescription explains manual context compaction to the model.
const ToolDescription = `Summarize the conversation so far and replace in-memory history with a compressed summary.
Use when the context is too large. You must call compact alone in this assistant message (no other tools in the same turn).
Actual compaction is performed by the runtime; this tool is a hint that triggers summarization.`

// New returns a placeholder tool so the model can request compaction by name. When compaction is enabled,
// enno.Agent implements summarization in the agent loop; the handler is only used if you wire the tool without Agent integration.
func New() enno.Tool {
	return enno.NewTypedTool(enno.CompactionToolName, ToolDescription, map[string]any{}, []string{}, func(ctx context.Context, _ struct{}) (string, error) {
		return "Compaction is handled by the enno runtime when compaction is enabled.", nil
	})
}
