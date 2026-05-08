package cliui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/dean2021/enno"
)

// ViewportString returns ANSI-styled transcript text for the bubbletea viewport.
// Line structure matches plainTextForSearch for jump-to-line search.
// width is the terminal width (used for user message background bars); pass 0 for a safe default.
func (s *mainViewState) ViewportString(width int) string {
	if s == nil || len(s.Messages) == 0 {
		return ""
	}
	var b strings.Builder
	for i, message := range s.Messages {
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(formatMessageLipgloss(message, width))
	}
	return b.String()
}

func formatMessageLipgloss(m chatMessage, width int) string {
	body := m.Message
	if !m.Rich {
		body = escapeTagLike(m.Message)
	} else {
		body = strings.TrimSpace(stripColorTags(m.Message))
	}
	if m.Author == "" {
		return lipgloss.NewStyle().Foreground(colorText).Render(body)
	}

	label := lipgloss.NewStyle().Foreground(authorLabelLipColor(m.Author)).Bold(true).Render(DisplayAuthor(m.Author) + ":")
	content := lipgloss.NewStyle().Foreground(colorText).Render(" " + body)

	if m.Author == "you" {
		w := width
		if w <= 0 {
			w = 80
		}
		line := label + content
		return lipgloss.NewStyle().
			Width(w).
			Background(colorUserMessageBG).
			Padding(0, 1).
			Render(line)
	}

	return label + content
}

// StatusLipgloss returns a single-line status string (with lipgloss styles).
func StatusLipgloss(event enno.Event) string {
	inactive := lipgloss.NewStyle().Foreground(colorInactive)
	switch event.Type {
	case enno.EventModelStart:
		return lipgloss.NewStyle().Foreground(colorWarning).Render(
			fmt.Sprintf("Calling model… round=%d messages=%d tools=%d", event.Round, event.MessageCount, event.ToolCount),
		)
	case enno.EventModelResponse:
		return lipgloss.NewStyle().Foreground(colorSuccess).Render("Model responded. ") +
			inactive.Render(formatUsage(event.Usage))
	case enno.EventToolStart:
		return lipgloss.NewStyle().Foreground(colorWarning).Render("Running tool ") +
			lipgloss.NewStyle().Foreground(colorToolAccent).Render(escapeTagLike(event.ToolCall.Name))
	case enno.EventToolResult:
		return lipgloss.NewStyle().Foreground(colorSuccess).Render("Tool complete ") +
			lipgloss.NewStyle().Foreground(colorToolAccent).Render(escapeTagLike(event.ToolCall.Name)) +
			" " + inactive.Render(event.Duration.Round(time.Millisecond).String())
	case enno.EventRoundComplete:
		return lipgloss.NewStyle().Foreground(colorSuccess).Render(
			fmt.Sprintf("Round %d complete. %s", event.Round, formatUsage(event.Usage)),
		)
	case enno.EventError:
		return lipgloss.NewStyle().Foreground(colorError).Render("Error: " + errorString(event.Err))
	default:
		return lipgloss.NewStyle().Foreground(colorSuccess).Render("Ready.")
	}
}

func statusLineReadyBubble(mouseEnabled bool) string {
	hint := "Tab transcript · Alt+↑↓ history · / search · Esc"
	if !mouseEnabled {
		hint = "Tab transcript · Alt+↑↓ history · PgUp/PgDn · Ctrl+↑↓ scroll · / search · Esc"
	}
	return lipgloss.NewStyle().Foreground(colorSuccess).Render("Ready.") + " " +
		lipgloss.NewStyle().Foreground(colorInactive).Render(hint)
}
