package cliui

import "strings"

// plainTextForSearch builds a newline-separated plain string aligned with how the
// transcript reads visually, without tview color tags, for substring search.
func plainTextForSearch(s *mainViewState) string {
	if s == nil {
		return ""
	}
	var b strings.Builder
	for _, message := range s.Messages {
		body := plainMessageBody(message)
		if message.Author == "" {
			b.WriteString(body)
			b.WriteString("\n\n")
			continue
		}
		b.WriteString(DisplayAuthor(message.Author))
		b.WriteString(": ")
		b.WriteString(body)
		b.WriteString("\n\n")
	}
	return b.String()
}

func plainMessageBody(m chatMessage) string {
	if m.Rich {
		return strings.TrimSpace(stripColorTags(m.Message))
	}
	return escapeTagLike(m.Message)
}

// lineOfFirstMatch returns the 0-based line index of the line containing the
// start of the first occurrence of query in plain. Lines are split by '\n'.
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
