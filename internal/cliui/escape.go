package cliui

import "regexp"

// bracketTagEscape escapes "[" ... "]" sequences so user text does not look like
// tview/lipgloss markup (aligned with rivo/tview Escape).
var bracketTagEscape = regexp.MustCompile(`(\[[a-zA-Z0-9_,;: \-\."#]+\[*)\]`)

func escapeTagLike(text string) string {
	return bracketTagEscape.ReplaceAllString(text, "$1[]")
}
