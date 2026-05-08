package cliui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/dean2021/enno"
)

func (s *mainViewState) ViewportString(width int, hoverLine int) string {
	s.buildRenderLines(width)
	return s.renderLinesString(width, hoverLine)
}

func (s *mainViewState) buildRenderLines(width int) {
	if s == nil || len(s.Messages) == 0 {
		s.msgStartLines = nil
		s.msgEndLines = nil
		s.renderLines = nil
		return
	}
	s.msgStartLines = make([]int, len(s.Messages))
	s.msgEndLines = make([]int, len(s.Messages))
	s.renderLines = nil
	for i, message := range s.Messages {
		if i > 0 {
			s.renderLines = append(s.renderLines, renderLine{
				MessageIndex: -1,
			})
		}
		s.msgStartLines[i] = len(s.renderLines)
		rendered := formatMessageLipgloss(message, width)
		rendered = wrapViewportMessage(rendered, width)
		lines := strings.Split(rendered, "\n")
		for _, line := range lines {
			s.renderLines = append(s.renderLines, renderLine{
				Text:         line,
				MessageIndex: i,
				Expandable:   message.FullContent != "",
			})
		}
		s.msgEndLines[i] = len(s.renderLines) - 1
	}
}

func (s *mainViewState) renderLinesString(width int, hoverLine int) string {
	if s == nil || len(s.renderLines) == 0 {
		return ""
	}
	var b strings.Builder
	for i, line := range s.renderLines {
		if i > 0 {
			b.WriteString("\n")
		}
		text := line.Text
		if i == hoverLine && line.Expandable {
			text = renderHoverHighlight(text, width)
		}
		b.WriteString(text)
	}
	return b.String()
}

func wrapViewportMessage(rendered string, width int) string {
	if width <= 0 {
		return rendered
	}
	return lipgloss.NewStyle().Width(width).Render(rendered)
}

func formatMessageLipgloss(m chatMessage, width int) string {
	if m.FullContent != "" && m.Expanded {
		return formatExpandedMessage(m)
	}

	body := m.Message
	if !m.Rich {
		body = escapeTagLike(m.Message)
	} else {
		body = strings.TrimSpace(stripColorTags(m.Message))
	}

	isExpandable := m.FullContent != "" && !m.Expanded

	expandHint := func(lines int) string {
		return " " + lipgloss.NewStyle().Foreground(colorInactive).Render(
			fmt.Sprintf("[%d lines \u00b7 click to expand]", lines))
	}

	if m.Author == "" {
		result := lipgloss.NewStyle().Foreground(colorText).Render("  " + body)
		if isExpandable {
			result += expandHint(strings.Count(m.FullContent, "\n") + 1)
		}
		return result
	}

	icon := authorIcon(m.Author)
	barColor := authorBarColor(m.Author)
	bar := lipgloss.NewStyle().Foreground(barColor).Render(icon + " ")
	label := lipgloss.NewStyle().Foreground(authorLabelLipColor(m.Author)).Bold(true).Render(DisplayAuthor(m.Author))

	if isExpandable {
		return bar + label + lipgloss.NewStyle().Foreground(colorText).Render(" "+body) + expandHint(strings.Count(m.FullContent, "\n")+1)
	}

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

func renderHoverHighlight(content string, width int) string {
	if width <= 0 {
		return lipgloss.NewStyle().Background(colorExpandHoverBG).Render(content)
	}
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if pad := width - lipgloss.Width(line); pad > 0 {
			lines[i] = line + strings.Repeat(" ", pad)
		}
	}
	return lipgloss.NewStyle().Background(colorExpandHoverBG).Render(strings.Join(lines, "\n"))
}

func formatExpandedMessage(m chatMessage) string {
	hint := lipgloss.NewStyle().Foreground(colorInactive).Render("[click to collapse]")
	fullBody := escapeTagLike(m.FullContent)

	if m.Author == "" {
		resultIcon := lipgloss.NewStyle().Foreground(colorResultDim).Render("\u25BE Result:")
		indented := indentLines(fullBody, "  ")
		content := lipgloss.NewStyle().Foreground(colorText).Render(indented)
		return resultIcon + "\n" + content + "\n" + hint
	}

	bar := lipgloss.NewStyle().Foreground(authorBarColor(m.Author)).Render("\u25BE ")
	label := lipgloss.NewStyle().Foreground(authorLabelLipColor(m.Author)).Bold(true).Render(DisplayAuthor(m.Author))
	content := lipgloss.NewStyle().Foreground(colorText).Render(fullBody)
	return bar + label + " " + content + "\n" + hint
}

func indentLines(text, prefix string) string {
	if text == "" {
		return ""
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
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
	hint := "Tab \u00b7 transcript  Alt+\u2191\u2193 \u00b7 history  / \u00b7 search  click \u00b7 expand  Esc \u00b7 quit"
	if !mouseEnabled {
		hint = "Tab \u00b7 transcript  Alt+\u2191\u2193 \u00b7 history  PgUp/PgDn  / \u00b7 search  Esc \u00b7 quit"
	}
	return icon + " " + lipgloss.NewStyle().Foreground(colorStatusReady).Render("Ready.") + "  " +
		lipgloss.NewStyle().Foreground(colorInactive).Render(hint)
}
