package enno

import (
	"os"
	"path/filepath"
)

const DefaultMaxToolRounds = 50

// CompactionConfig enables optional context compaction (micro-trimming of old tool
// results, automatic summarization when estimated input size exceeds a threshold,
// and the manual compact tool). Nil in Config means compaction is disabled.
type CompactionConfig struct {
	Enabled bool

	// TranscriptDir stores JSONL transcripts before summarization. Empty uses
	// ~/.enno/transcripts when Enabled (after withDefaults).
	TranscriptDir string

	// ModelContextTokens, when positive, sets auto-compact threshold to
	// ModelContextTokens - AutoCompactBufferTokens (buffer defaults to 13000 in withDefaults).
	// Takes precedence over AutoCompactInputTokens when the difference is positive.
	ModelContextTokens int64

	// AutoCompactBufferTokens is subtracted from ModelContextTokens for the effective threshold.
	// Ignored when ModelContextTokens is zero. Zero defaults to 13000 when ModelContextTokens > 0.
	AutoCompactBufferTokens int64

	// AutoCompactInputTokens triggers summarization when estimated/conservative input tokens
	// meet or exceed this value. Zero defaults to 50000 when ModelContextTokens is zero.
	AutoCompactInputTokens int64

	// KeepRecentToolResults is how many latest eligible RoleTool messages keep full content in Micro.
	// Zero defaults to 3.
	KeepRecentToolResults int

	// MicroCompactMinChars replaces longer RoleTool contents with a placeholder. Zero defaults to 100.
	MicroCompactMinChars int

	// MicroCompactToolNames, when non-empty, limits Micro compaction to tool results whose tool name
	// is in this list. Empty means all RoleTool messages participate (legacy behavior).
	MicroCompactToolNames []string

	// SkipOnSummarizeError, when true, causes automatic compaction to log an error event and continue
	// without replacing history if summarization fails. Manual compact via the compact tool remains strict.
	SkipOnSummarizeError bool
}

func (c CompactionConfig) withDefaults() CompactionConfig {
	if c.ModelContextTokens > 0 && c.AutoCompactBufferTokens <= 0 {
		c.AutoCompactBufferTokens = defaultAutoCompactBufferTokens
	}
	if c.AutoCompactInputTokens <= 0 {
		c.AutoCompactInputTokens = defaultAutoCompactInputTokens
	}
	if c.KeepRecentToolResults <= 0 {
		c.KeepRecentToolResults = defaultKeepRecentToolResults
	}
	if c.MicroCompactMinChars <= 0 {
		c.MicroCompactMinChars = defaultMicroCompactMinChars
	}
	if c.Enabled && c.TranscriptDir == "" {
		if home, err := os.UserHomeDir(); err == nil {
			c.TranscriptDir = filepath.Join(home, ".enno", "transcripts")
		}
	}
	return c
}

type Config struct {
	Provider      Provider
	SystemPrompt  string
	Tools         []Tool
	MaxToolRounds int
	EventHandler  EventHandler
	Compaction    *CompactionConfig
}

func (c Config) withDefaults() Config {
	if c.MaxToolRounds <= 0 {
		c.MaxToolRounds = DefaultMaxToolRounds
	}
	if c.Compaction != nil {
		cc := c.Compaction.withDefaults()
		c.Compaction = &cc
	}
	return c
}
