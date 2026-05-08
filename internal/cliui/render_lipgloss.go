package cliui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/dean2021/enno"
)

func (s *mainViewState) ViewportString(width int) string {
	if s == nil || len(s.Messages) == 0 {
		return ""
	}
	var b strings.Builder
	for i, message := range s.Messages {
		if i > 0 {
			b.WriteString("\n")
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
		return lipgloss.NewStyle().Foreground(colorResultDim).Render("  " + body)
	}

	icon := authorIcon(m.Author)
	barColor := authorBarColor(m.Author)
	bar := lipgloss.NewStyle().Foreground(barColor).Render(icon + " ")
	label := lipgloss.NewStyle().Foreground(authorLabelLipColor(m.Author)).Bold(true).Render(DisplayAuthor(m.Author))

	if m.Author == "you" {
		w := width
		if w <= 0 {
			w = 80
		}
		inner := bar + label + lipgloss.NewStyle().Foreground(colorText).Render(" "+body)
		return lipgloss.NewStyle().
			Width(w).
			Background(colorUserMessageBG).
			Padding(0, 1).
			Render(inner)
	}

	if m.Author == "tool" {
		return bar + label + lipgloss.NewStyle().Foreground(colorToolMuted).Render(" "+body)
	}

	if m.Author == "error" {
		return bar + label + lipgloss.NewStyle().Foreground(colorError).Render(" "+body)
	}

	return bar + label + lipgloss.NewStyle().Foreground(colorText).Render(" "+body)
}

func StatusLipgloss(event enno.Event) string {
	inactive := lipgloss.NewStyle().Foreground(colorInactive)
	switch event.Type {
	case enno.EventModelStart:
		return lipgloss.NewStyle().Foreground(colorStatusBusy).Render("\u29D7") +
			" " + lipgloss.NewStyle().Foreground(colorWarning).Render(
			fmt.Sprintf("Calling model\u2026 round=%d messages=%d tools=%d", event.Round, event.MessageCount, event.ToolCount),
		)
	case enno.EventModelResponse:
		return lipgloss.NewStyle().Foreground(colorSuccess).Render("\u2713") +
			" " + lipgloss.NewStyle().Foreground(colorSuccess).Render("Model responded") +
			" " + lipgloss.NewStyle().Foreground(colorInactive).Render(event.Duration.Round(time.Millisecond).String()) +
			" " + inactive.Render(formatUsage(event.Usage))
	case enno.EventToolStart:
		return lipgloss.NewStyle().Foreground(colorStatusBusy).Render("\u25B6") +
			" " + lipgloss.NewStyle().Foreground(colorWarning).Render("Running ") +
			lipgloss.NewStyle().Foreground(colorToolAccent).Bold(true).Render(escapeTagLike(event.ToolCall.Name))
	case enno.EventToolResult:
		return lipgloss.NewStyle().Foreground(colorSuccess).Render("\u2713") +
			" " + lipgloss.NewStyle().Foreground(colorSuccess).Render("Tool complete") +
			" " + lipgloss.NewStyle().Foreground(colorToolAccent).Render(escapeTagLike(event.ToolCall.Name)) +
			" " + inactive.Render(event.Duration.Round(time.Millisecond).String())
	case enno.EventRoundComplete:
		return lipgloss.NewStyle().Foreground(colorSuccess).Render("\u2713") +
			" " + lipgloss.NewStyle().Foreground(colorSuccess).Render(
			fmt.Sprintf("Round %d complete. %s", event.Round, formatUsage(event.Usage)),
		)
	case enno.EventError:
		return lipgloss.NewStyle().Foreground(colorError).Render("\u2718") +
			" " + lipgloss.NewStyle().Foreground(colorError).Render("Error: "+errorString(event.Err))
	default:
		return lipgloss.NewStyle().Foreground(colorStatusReady).Render("\u2713") +
			" " + lipgloss.NewStyle().Foreground(colorStatusReady).Render("Ready.")
	}
}

func statusLineReadyBubble(mouseEnabled bool) string {
	icon := lipgloss.NewStyle().Foreground(colorStatusReady).Render("\u2713")
	hint := "Tab \u2022 transcript  \u2191\u2193 \u2022 history  / \u2022 search  Esc \u2022 quit"
	if !mouseEnabled {
		hint = "Tab \u2022 transcript  Alt+\u2191\u2193 \u2022 history  PgUp/PgDn  \u2191\u2193 \u2022 scroll  / \u2022 search  Esc \u2022 quit"
	}
	return icon + " " + lipgloss.NewStyle().Foreground(colorStatusReady).Render("Ready.") + "  " +
		lipgloss.NewStyle().Foreground(colorInactive).Render(hint)
}
