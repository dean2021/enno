package enno

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CompactionToolName is the registered tool name for manual compaction (same string used by tools/compact).
const CompactionToolName = "compact"

const (
	defaultAutoCompactInputTokens int64 = 50000
	defaultKeepRecentToolResults        = 3
	defaultMicroCompactMinChars         = 100
	maxSummarizePayloadChars            = 80000
)

const compactionSummarySystemPrompt = `You summarize a coding assistant conversation transcript so the assistant can continue with minimal context loss.
Include: main goals, files and paths touched, commands run and outcomes, errors, open tasks, and user preferences mentioned.
Do not invent facts. Output plain text only, no JSON or markdown code fences unless quoting paths.`

func microCompact(messages []Message, keepRecent, minChars int) {
	if keepRecent <= 0 {
		keepRecent = defaultKeepRecentToolResults
	}
	if minChars <= 0 {
		minChars = defaultMicroCompactMinChars
	}
	var toolIdx []int
	for i := range messages {
		if messages[i].Role == RoleTool {
			toolIdx = append(toolIdx, i)
		}
	}
	if len(toolIdx) <= keepRecent {
		return
	}
	old := toolIdx[:len(toolIdx)-keepRecent]
	for _, i := range old {
		if len(messages[i].Content) <= minChars {
			continue
		}
		name := toolNameForCompaction(messages, i)
		messages[i].Content = fmt.Sprintf("[Previous: used %s]", name)
	}
}

func toolNameForCompaction(messages []Message, toolIdx int) string {
	id := messages[toolIdx].ToolCallID
	for j := toolIdx - 1; j >= 0; j-- {
		if messages[j].Role != RoleAssistant {
			continue
		}
		for _, tc := range messages[j].ToolCalls {
			if tc.ID == id {
				return tc.Name
			}
		}
	}
	return "unknown"
}

func shouldAutoCompact(req Request, threshold int64) bool {
	if threshold <= 0 {
		threshold = defaultAutoCompactInputTokens
	}
	u := EstimateUsage(req)
	return u.InputTokens >= threshold
}

func saveCompactionTranscript(dir string, messages []Message) (path string, err error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path = filepath.Join(dir, fmt.Sprintf("transcript_%d.jsonl", time.Now().Unix()))
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	for _, m := range messages {
		line, err := json.Marshal(m)
		if err != nil {
			return path, err
		}
		if _, err := f.Write(append(line, '\n')); err != nil {
			return path, err
		}
	}
	return path, nil
}

func messagesToPayloadForSummarize(messages []Message, maxChars int) string {
	if maxChars <= 0 {
		maxChars = maxSummarizePayloadChars
	}
	var b strings.Builder
	for _, m := range messages {
		line, err := json.Marshal(m)
		if err != nil {
			continue
		}
		if b.Len()+len(line)+1 > maxChars {
			b.WriteString("\n... [truncated for summarization]")
			break
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.Write(line)
	}
	return b.String()
}

func summarizeTranscript(ctx context.Context, provider Provider, transcriptPayload string) (string, error) {
	transcriptPayload = strings.TrimSpace(transcriptPayload)
	if transcriptPayload == "" {
		return "(empty transcript)", nil
	}
	req := Request{
		SystemPrompt: compactionSummarySystemPrompt,
		Messages: []Message{
			UserMessage("Summarize the following conversation transcript for continuation:\n\n" + transcriptPayload),
		},
		Tools: nil,
	}
	resp, err := provider.Complete(ctx, req)
	if err != nil {
		return "", err
	}
	out := strings.TrimSpace(resp.Content)
	if out == "" {
		return "(no summary)", nil
	}
	return out, nil
}

func compressedUserContent(summary string) string {
	return "[Compressed]\n\n" + summary
}
