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

	// AutoCompactInputTokens triggers summarization when EstimateUsage(req).InputTokens
	// meets or exceeds this value. Zero defaults to 50000.
	AutoCompactInputTokens int64

	// KeepRecentToolResults is how many latest RoleTool messages keep full content in Micro.
	// Zero defaults to 3.
	KeepRecentToolResults int

	// MicroCompactMinChars replaces longer RoleTool contents with a placeholder. Zero defaults to 100.
	MicroCompactMinChars int
}

func (c CompactionConfig) withDefaults() CompactionConfig {
	if c.AutoCompactInputTokens <= 0 {
		c.AutoCompactInputTokens = 50000
	}
	if c.KeepRecentToolResults <= 0 {
		c.KeepRecentToolResults = 3
	}
	if c.MicroCompactMinChars <= 0 {
		c.MicroCompactMinChars = 100
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
