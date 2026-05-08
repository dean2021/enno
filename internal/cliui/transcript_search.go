package cliui

import "strings"

func lineOfFirstMatch(plain, query string) (line int, ok bool) {
	query = strings.TrimSpace(query)
	if plain == "" || query == "" {
		return 0, false
	}
	idx := strings.Index(plain, query)
	if idx < 0 {
		return 0, false
	}
	return strings.Count(plain[:idx], "\n"), true
}
