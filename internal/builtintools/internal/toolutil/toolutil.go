package toolutil

import "time"

const (
	DefaultTimeout          = 120 * time.Second
	DefaultMaxOutputChars   = 50000
	DefaultTruncationSuffix = "\n\n[truncated]"
)

func Timeout(value time.Duration) time.Duration {
	if value <= 0 {
		return DefaultTimeout
	}
	return value
}

func MaxOutputChars(value int) int {
	if value <= 0 {
		return DefaultMaxOutputChars
	}
	return value
}

func TruncateRunes(text string, maxRunes int, suffix string) string {
	if maxRunes <= 0 {
		return text
	}
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	if suffix == "" {
		suffix = DefaultTruncationSuffix
	}
	suffixRunes := []rune(suffix)
	if len(suffixRunes) >= maxRunes {
		return string(runes[:maxRunes])
	}
	return string(runes[:maxRunes-len(suffixRunes)]) + suffix
}
