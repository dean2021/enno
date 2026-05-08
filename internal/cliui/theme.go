package cliui

import "github.com/charmbracelet/lipgloss"

var (
	colorBriefLabelYou       = lipgloss.Color("#93C5FD")
	colorBriefLabelAssistant = lipgloss.Color("#C4B5FD")
	colorUserMessageBG       = lipgloss.Color("#1E1B4B")
	colorUserBarFG           = lipgloss.Color("#60A5FA")
	colorText                = lipgloss.Color("#E2E8F0")
	colorToolMuted           = lipgloss.Color("#94A3B8")
	colorToolAccent          = lipgloss.Color("#38BDF8")
	colorError               = lipgloss.Color("#F87171")
	colorSuccess             = lipgloss.Color("#4ADE80")
	colorWarning             = lipgloss.Color("#FBBF24")
	colorInactive            = lipgloss.Color("#64748B")
	colorSubtleBorder        = lipgloss.Color("#334155")
	colorPromptBorder        = lipgloss.Color("#475569")
	colorDimText             = lipgloss.Color("#94A3B8")
	colorResultDim           = lipgloss.Color("#64748B")
	colorResultExpandedBG    = lipgloss.Color("#334155")
	colorAssistantBG         = lipgloss.Color("#1A1A2E")
	colorToolBarFG           = lipgloss.Color("#38BDF8")
	colorErrorBarFG          = lipgloss.Color("#F87171")
	colorThinkingFG          = lipgloss.Color("#FBBF24")
	colorInputBorder         = lipgloss.Color("#6366F1")
	colorInputFocusBorder    = lipgloss.Color("#818CF8")
	colorInputPrompt         = lipgloss.Color("#818CF8")
	colorStatusReady         = lipgloss.Color("#4ADE80")
	colorStatusBusy          = lipgloss.Color("#FBBF24")
	colorSearchBorder        = lipgloss.Color("#6366F1")
	colorExpandHoverBG       = lipgloss.Color("#1E293B")
)

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

func authorIcon(author string) string {
	switch author {
	case "you":
		return "\u25B8" // ▸
	case "enno":
		return "\u276F" // ❯
	case "tool":
		return "\u25CF" // ●
	case "error":
		return "\u2718" // ✘
	default:
		return "\u25CB" // ○
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

func authorBarColor(author string) lipgloss.Color {
	switch author {
	case "you":
		return colorUserBarFG
	case "enno":
		return colorBriefLabelAssistant
	case "tool":
		return colorToolBarFG
	case "error":
		return colorErrorBarFG
	default:
		return colorInactive
	}
}
