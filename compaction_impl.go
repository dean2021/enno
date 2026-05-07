package enno

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// CompactionToolName is the registered tool name for manual compaction (same string used by tools/compact).
const CompactionToolName = "compact"

const (
	defaultAutoCompactInputTokens    = 50000
	defaultAutoCompactBufferTokens   = 13000
	defaultKeepRecentToolResults     = 3
	defaultMicroCompactMinChars      = 100
	maxSummarizePayloadChars         = 80000
	maxConsecutiveCompactionFailures = 3
)

const compactionSummarySystemPrompt = `You produce a detailed summary of the assistant conversation transcript so work can continue without losing technical context.

Respond with TEXT ONLY (no tool calls). Use two XML regions:

1. First <analysis>...</analysis>: organize chronologically what happened—user goals, files touched, commands, errors, fixes, pending work.

2. Then <summary>...</summary> with numbered sections:
   1. Primary Request and Intent
   2. Key Technical Concepts
   3. Files and Code Sections (paths and short rationale)
   4. Errors and Fixes
   5. Problem Solving
   6. User Messages (non-tool user text that matters)
   7. Pending Tasks
   8. Current Work
   9. Optional Next Step

Do not invent facts. Plain text inside tags; avoid markdown code fences except short path or snippet quotes when essential.`

var (
	reCompactionAnalysis = regexp.MustCompile(`(?s)<analysis>.*?</analysis>`)
	reCompactionSummary  = regexp.MustCompile(`(?s)<summary>(.*?)</summary>`)
	reMultiNewline       = regexp.MustCompile(`\n{3,}`)
)

// FormatCompactSummary strips <analysis>, extracts <summary> body when present, and normalizes whitespace.
// If no <summary> tag is found, returns the whole string trimmed (backward compatible).
func FormatCompactSummary(raw string) string {
	s := strings.TrimSpace(raw)
	s = reCompactionAnalysis.ReplaceAllString(s, "")
	s = strings.TrimSpace(s)
	if m := reCompactionSummary.FindStringSubmatch(s); len(m) > 1 {
		inner := strings.TrimSpace(m[1])
		return strings.TrimSpace("Summary:\n" + inner)
	}
	s = reCompactionSummary.ReplaceAllString(s, "")
	return strings.TrimSpace(reMultiNewline.ReplaceAllString(s, "\n\n"))
}

// effectiveAutoCompactThreshold returns the token threshold for triggering automatic compaction.
// Precedence: if ModelContextTokens > 0, use ModelContextTokens - AutoCompactBufferTokens when positive;
// otherwise AutoCompactInputTokens (after defaults), else package default.
func effectiveAutoCompactThreshold(cfg CompactionConfig) int64 {
	if cfg.ModelContextTokens > 0 {
		buf := cfg.AutoCompactBufferTokens
		if buf <= 0 {
			buf = defaultAutoCompactBufferTokens
		}
		if t := cfg.ModelContextTokens - buf; t > 0 {
			return t
		}
	}
	if cfg.AutoCompactInputTokens > 0 {
		return cfg.AutoCompactInputTokens
	}
	return defaultAutoCompactInputTokens
}

// inputTokensOverThreshold compares conservative input size to the effective threshold.
func inputTokensOverThreshold(req Request, cfg CompactionConfig, lastCompleteInputTokens int64) bool {
	threshold := effectiveAutoCompactThreshold(cfg)
	est := EstimateUsage(req).InputTokens
	input := est
	if lastCompleteInputTokens > input {
		input = lastCompleteInputTokens
	}
	return input >= threshold
}

func microCompact(messages []Message, keepRecent, minChars int, microToolNames []string) {
	if keepRecent <= 0 {
		keepRecent = defaultKeepRecentToolResults
	}
	if minChars <= 0 {
		minChars = defaultMicroCompactMinChars
	}
	whitelist := make(map[string]bool, len(microToolNames))
	for _, n := range microToolNames {
		n = strings.TrimSpace(n)
		if n != "" {
			whitelist[n] = true
		}
	}
	useWhitelist := len(whitelist) > 0

	var toolIdx []int
	for i := range messages {
		if messages[i].Role != RoleTool {
			continue
		}
		name := toolNameForCompaction(messages, i)
		if useWhitelist && !whitelist[name] {
			continue
		}
		toolIdx = append(toolIdx, i)
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

func completeCompactionSummarize(ctx context.Context, provider Provider, transcriptPayload string) (string, error) {
	transcriptPayload = strings.TrimSpace(transcriptPayload)
	if transcriptPayload == "" {
		return "(empty transcript)", nil
	}
	req := Request{
		SystemPrompt: compactionSummarySystemPrompt,
		Messages: []Message{
			UserMessage("Summarize the following conversation transcript for continuation.\n\n" + transcriptPayload),
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

// summarizeCompaction runs one summarization call; on failure retries once with the second half of messages (JSONL).
func summarizeCompaction(ctx context.Context, provider Provider, messages []Message) (string, error) {
	payload := messagesToPayloadForSummarize(messages, 0)
	raw, err := completeCompactionSummarize(ctx, provider, payload)
	if err == nil {
		return FormatCompactSummary(raw), nil
	}
	if len(messages) <= 1 {
		return "", err
	}
	half := len(messages) / 2
	if half < 1 {
		half = 1
	}
	tail := messages[half:]
	payload2 := messagesToPayloadForSummarize(tail, 0)
	raw2, err2 := completeCompactionSummarize(ctx, provider, payload2)
	if err2 != nil {
		return "", err
	}
	return FormatCompactSummary(raw2), nil
}

func compressedUserContent(summary string) string {
	return "[Compressed]\n\n" + summary
}
