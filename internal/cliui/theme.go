package cliui

import "github.com/charmbracelet/lipgloss"

// Claude Code–inspired dark semantic palette (see Anthropic terminal-config theme tokens).
var (
	colorBriefLabelYou       = lipgloss.Color("#93C5FD") // briefLabelYou
	colorBriefLabelAssistant = lipgloss.Color("#C4B5FD") // claude / assistant accent
	colorUserMessageBG       = lipgloss.Color("#1E1B4B") // userMessageBackground
	colorText                = lipgloss.Color("#E2E8F0") // default foreground on dark
	colorToolMuted           = lipgloss.Color("#94A3B8")
	colorToolAccent          = lipgloss.Color("#38BDF8")
	colorError               = lipgloss.Color("#F87171")
	colorSuccess             = lipgloss.Color("#4ADE80")
	colorWarning             = lipgloss.Color("#FBBF24")
	colorInactive            = lipgloss.Color("#64748B")
	colorSubtleBorder        = lipgloss.Color("#334155") // subtle
	colorPromptBorder        = lipgloss.Color("#475569") // promptBorder
)

// DisplayAuthor maps internal author keys to transcript labels (matches Claude Code You / assistant brief labels).
func DisplayAuthor(author string) string {
	switch author {
	case "you":
		return "You"
	case "enno":
		return "Enno"
	case "tool":
		return "Tool"
	case "error":
		return "Error"
	default:
		return author
	}
}

func authorLabelLipColor(author string) lipgloss.Color {
	switch author {
	case "you":
		return colorBriefLabelYou
	case "enno":
		return colorBriefLabelAssistant
	case "error":
		return colorError
	case "tool":
		return colorToolMuted
	default:
		return colorText
	}
}
