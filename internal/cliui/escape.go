package cliui

import "regexp"

var bracketTagEscape = regexp.MustCompile(`(\[[a-zA-Z0-9_,;: \-\."#]+\[*)\]`)

func escapeTagLike(text string) string {
	return bracketTagEscape.ReplaceAllString(text, "$1[]")
}

var ansiEscapeRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripANSI(s string) string {
	return ansiEscapeRe.ReplaceAllString(s, "")
}
