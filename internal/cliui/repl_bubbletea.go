package cliui

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dean2021/enno"
	"github.com/dean2021/enno/internal/history"
)

type focusArea int

const (
	focusPrompt focusArea = iota
	focusTranscript
	focusSearch
)

type agentEventWrap struct {
	ev enno.Event
}

type runFinishedMsg struct {
	result enno.RunResult
	err    error
}

type restoreStatusMsg struct{}

type bubbleModel struct {
	width, height int
	statusLine    string
	vp            viewport.Model
	ti            textinput.Model
	searchTI      textinput.Model
	focus         focusArea
	mainState     *mainViewState
	followOutput  bool
	hist          *inputHistory
	busy          bool
	events        <-chan enno.Event
	agent         *enno.Agent
	session       *enno.Session
	config        Config
	ctx           context.Context
	cancel        context.CancelFunc
	prog          *tea.Program
	ggDeadline    time.Time
	searchReturn  focusArea
	startEvents   sync.Once
	disableMouse  bool
	plainContent  string
	hoverLine     int
	lastMouseY    int
	hasMouseY     bool
}

func bubbleteaREPL(ctx context.Context, agent *enno.Agent, config Config) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	mainState := newMainViewState()
	mainState.AppendMessage("enno", "Interactive TUI started.")

	var histEntries []string
	if config.Recorder != nil {
		entries, err := history.LoadRecent(config.Recorder.Path(), 500)
		if err == nil {
			for _, e := range entries {
				if e.Display != "" {
					histEntries = append(histEntries, e.Display)
				}
			}
		}
	}

	ti := textinput.New()
	ti.Prompt = lipgloss.NewStyle().Foreground(colorInputPrompt).Render("\u276F") + " "
	ti.CharLimit = 0

	sti := textinput.New()
	sti.Prompt = "/ "
	sti.CharLimit = 0

	vp := viewport.New(80, 20)
	vp.MouseWheelEnabled = !config.DisableMouse
	vp.MouseWheelDelta = 3

	m := &bubbleModel{
		statusLine:   statusLineReadyBubble(!config.DisableMouse),
		vp:           vp,
		ti:           ti,
		searchTI:     sti,
		focus:        focusPrompt,
		mainState:    mainState,
		followOutput: true,
		hist:         newInputHistory(histEntries),
		events:       config.Events,
		agent:        agent,
		session:      config.Session,
		config:       config,
		ctx:          ctx,
		cancel:       cancel,
		disableMouse: config.DisableMouse,
		hoverLine:    -1,
	}
	m.syncViewport()

	opts := []tea.ProgramOption{
		tea.WithAltScreen(),
		tea.WithContext(ctx),
	}
	if !config.DisableMouse {
		opts = append(opts, tea.WithMouseAllMotion())
	}
	p := tea.NewProgram(m, opts...)
	m.prog = p
	_, err := p.Run()
	return err
}

func (m *bubbleModel) startEventPump() {
	m.startEvents.Do(func() {
		if m.events == nil {
			return
		}
		go func() {
			for {
				select {
				case <-m.ctx.Done():
					return
				case ev, ok := <-m.events:
					if !ok {
						return
					}
					if m.prog != nil {
						m.prog.Send(agentEventWrap{ev})
					}
				}
			}
		}()
	})
}

func (m *bubbleModel) Init() tea.Cmd {
	m.startEventPump()
	return m.ti.Focus()
}

func (m *bubbleModel) syncViewport() {
	rendered := m.mainState.ViewportString(m.vp.Width, -1)
	m.vp.SetContent(rendered)
	if m.followOutput {
		m.vp.GotoBottom()
	}

	m.hoverLine = -1
	if m.hasMouseY {
		if contentLine, ok := m.transcriptContentLine(m.lastMouseY); ok {
			m.hoverLine = contentLine
		}
	}

	rendered = m.mainState.ViewportString(m.vp.Width, m.hoverLine)
	m.vp.SetContent(rendered)
	if m.followOutput {
		m.vp.GotoBottom()
	}
	m.plainContent = stripANSI(rendered)
}

func (m *bubbleModel) layout() {
	statusH := 1
	promptBoxH := 3
	frameH := 2
	frameW := 2
	if m.width <= 0 || m.height <= 0 {
		return
	}
	vpBoxH := m.height - statusH - promptBoxH
	if vpBoxH < frameH+1 {
		vpBoxH = frameH + 1
	}
	m.vp.Width = max(10, m.width-frameW)
	m.vp.Height = max(1, vpBoxH-frameH)
	m.ti.Width = max(10, m.width-4)
	m.searchTI.Width = min(60, m.width-4)
	m.syncViewport()
}

func (m *bubbleModel) pageStep() int {
	h := m.vp.Height
	if h > 3 {
		n := h - 2
		if n >= 4 {
			return n
		}
	}
	return 12
}

func (m *bubbleModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.layout()
		return m, nil

	case agentEventWrap:
		m.mainState.AppendEvent(msg.ev)
		m.statusLine = StatusLipgloss(msg.ev)
		m.syncViewport()
		return m, nil

	case runFinishedMsg:
		m.busy = false
		m.statusLine = statusLineReadyBubble(!m.disableMouse)
		if msg.err != nil {
			if strings.TrimSpace(msg.result.Content) != "" {
				m.mainState.AppendMessage("enno", msg.result.Content)
			}
			m.mainState.AppendMessage("error", msg.err.Error())
		} else if strings.TrimSpace(msg.result.Content) == "" {
			m.mainState.AppendMessage("enno", "(no response)")
		} else {
			m.mainState.AppendMessage("enno", msg.result.Content)
		}
		m.followOutput = true
		m.syncViewport()
		return m, nil

	case restoreStatusMsg:
		if !m.busy {
			m.statusLine = statusLineReadyBubble(!m.disableMouse)
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.MouseMsg:
		return m.handleMouse(msg)
	}

	return m, nil
}

func (m *bubbleModel) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch msg.Button {
	case tea.MouseButtonWheelUp, tea.MouseButtonWheelDown,
		tea.MouseButtonWheelLeft, tea.MouseButtonWheelRight:
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown {
			m.followOutput = m.vp.AtBottom()
		}
		m.updateHover(msg.Y)
		return m, cmd
	case tea.MouseButtonLeft:
		switch msg.Action {
		case tea.MouseActionPress:
			clickLine, ok := m.transcriptContentLine(msg.Y)
			if !ok {
				return m, nil
			}
			if m.mainState.ToggleExpandAtLine(clickLine) {
				m.syncViewport()
			}
		case tea.MouseActionMotion:
			m.updateHover(msg.Y)
		}
		return m, nil
	default:
		if msg.Action == tea.MouseActionMotion {
			m.updateHover(msg.Y)
			return m, nil
		}
		if m.focus != focusTranscript {
			return m, nil
		}
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd
	}
}

func (m *bubbleModel) updateHover(mouseY int) {
	m.lastMouseY = mouseY
	m.hasMouseY = true
	hoverLine := -1
	if contentLine, ok := m.transcriptContentLine(mouseY); ok {
		hoverLine = contentLine
	}
	if m.hoverLine != hoverLine {
		m.hoverLine = hoverLine
		m.syncViewport()
	}
}

func (m *bubbleModel) transcriptContentLine(screenY int) (int, bool) {
	const viewportContentTop = 1 // top border occupies row 0
	contentRow := screenY - viewportContentTop
	if contentRow < 0 || contentRow >= m.vp.Height {
		return 0, false
	}
	return m.vp.YOffset + contentRow, true
}

func (m *bubbleModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.focus == focusSearch {
		switch msg.String() {
		case "esc":
			return m.closeSearch()
		case "enter", "ctrl+enter":
			return m.execSearch()
		default:
			sti, cmd := m.searchTI.Update(msg)
			m.searchTI = sti
			return m, cmd
		}
	}

	if m.focus == focusTranscript {
		return m.handleTranscriptKeys(msg)
	}

	// focusPrompt
	if msg.Type == tea.KeyCtrlC {
		m.cancel()
		return m, tea.Quit
	}
	if msg.Type == tea.KeyEsc {
		m.cancel()
		return m, tea.Quit
	}
	if key.Matches(msg, key.NewBinding(key.WithKeys("tab"))) {
		m.ti.Blur()
		m.focus = focusTranscript
		return m, nil
	}
	if msg.String() == "ctrl+f" {
		return m.openSearch()
	}
	if msg.Type == tea.KeyUp && msg.Alt {
		if s, ok := m.hist.Up(); ok {
			m.ti.SetValue(s)
		}
		return m, nil
	}
	if msg.Type == tea.KeyDown && msg.Alt {
		if s, ok := m.hist.Down(); ok {
			m.ti.SetValue(s)
		}
		return m, nil
	}
	if (msg.Type == tea.KeyUp || msg.Type == tea.KeyDown) && !msg.Alt {
		// Swallow synthesized arrows from trackpads when not Alt+arrow.
		return m, nil
	}
	if msg.Type == tea.KeyCtrlUp {
		m.followOutput = false
		m.vp.LineUp(3)
		return m, nil
	}
	if msg.Type == tea.KeyCtrlDown {
		m.vp.LineDown(3)
		m.followOutput = m.vp.AtBottom()
		return m, nil
	}
	if msg.Type == tea.KeyEnter {
		return m.submitPrompt()
	}
	ti, cmd := m.histNoteThenUpdate(msg)
	m.ti = ti
	return m, cmd
}

func (m *bubbleModel) histNoteThenUpdate(msg tea.KeyMsg) (textinput.Model, tea.Cmd) {
	m.hist.ResetDraft(m.ti.Value())
	return m.ti.Update(msg)
}

func (m *bubbleModel) handleTranscriptKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab", "shift+tab":
		m.focus = focusPrompt
		return m, m.ti.Focus()
	case "esc":
		m.focus = focusPrompt
		return m, m.ti.Focus()
	case "ctrl+f":
		return m.openSearch()
	}

	// gg / G / /
	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 {
		switch msg.Runes[0] {
		case '/':
			return m.openSearch()
		case 'G':
			m.followOutput = true
			m.vp.GotoBottom()
			return m, nil
		case 'g':
			if !m.ggDeadline.IsZero() && time.Now().Before(m.ggDeadline) {
				m.followOutput = false
				m.vp.GotoTop()
				m.ggDeadline = time.Time{}
			} else {
				m.ggDeadline = time.Now().Add(500 * time.Millisecond)
			}
			return m, nil
		}
	}
	m.ggDeadline = time.Time{}

	switch msg.Type {
	case tea.KeyHome:
		m.followOutput = false
		m.vp.GotoTop()
		return m, nil
	case tea.KeyEnd:
		m.followOutput = true
		m.vp.GotoBottom()
		return m, nil
	}

	switch msg.Type {
	case tea.KeyPgUp:
		m.followOutput = false
		m.vp.PageUp()
		return m, nil
	case tea.KeyPgDown:
		m.vp.PageDown()
		m.followOutput = m.vp.AtBottom()
		return m, nil
	}

	vp, cmd := m.vp.Update(msg)
	m.vp = vp
	if msg.Type == tea.KeyUp || msg.Type == tea.KeyDown {
		m.followOutput = m.vp.AtBottom()
	}
	return m, cmd
}

func (m *bubbleModel) openSearch() (tea.Model, tea.Cmd) {
	m.searchReturn = m.focus
	m.focus = focusSearch
	m.searchTI.SetValue("")
	sCmd := m.searchTI.Focus()
	m.ti.Blur()
	return m, sCmd
}

func (m *bubbleModel) closeSearch() (tea.Model, tea.Cmd) {
	m.focus = m.searchReturn
	m.searchTI.Blur()
	if m.focus == focusPrompt {
		return m, m.ti.Focus()
	}
	m.ti.Blur()
	return m, nil
}

func (m *bubbleModel) execSearch() (tea.Model, tea.Cmd) {
	q := strings.TrimSpace(m.searchTI.Value())
	if q == "" {
		return m.closeSearch()
	}
	plain := m.plainContent
	if line, ok := lineOfFirstMatch(plain, q); ok {
		m.followOutput = false
		m.vp.SetYOffset(line)
	} else {
		m.statusLine = lipgloss.NewStyle().Foreground(colorError).Render("No match.") + " " + lipgloss.NewStyle().Foreground(colorInactive).Render("Try another substring.")
		go func() {
			time.Sleep(2 * time.Second)
			if m.prog != nil {
				m.prog.Send(restoreStatusMsg{})
			}
		}()
	}
	m.focus = focusTranscript
	m.searchTI.Blur()
	m.ti.Blur()
	return m, nil
}

func (m *bubbleModel) submitPrompt() (tea.Model, tea.Cmd) {
	if m.busy {
		return m, nil
	}
	query := strings.TrimSpace(m.ti.Value())
	if query == "" {
		return m, nil
	}
	if shouldExit(query) {
		m.cancel()
		return m, tea.Quit
	}
	m.busy = true
	m.ti.SetValue("")
	m.statusLine = lipgloss.NewStyle().Foreground(colorWarning).Render("Waiting for model…")
	m.mainState.AppendMessage("you", query)
	m.followOutput = true
	m.syncViewport()
	m.hist.Append(query)
	if m.config.Recorder != nil {
		_ = m.config.Recorder.Record(query)
	}
	prog := m.prog
	agent := m.agent
	session := m.session
	ctx := m.ctx
	go func() {
		result, err := agent.RunSession(ctx, session, query)
		if prog != nil {
			prog.Send(runFinishedMsg{result: result, err: err})
		}
	}()
	return m, nil
}

func (m *bubbleModel) View() string {
	status := lipgloss.NewStyle().Width(m.width).Padding(0, 1).Render(m.statusLine)

	viewportStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorSubtleBorder).
		Width(m.vp.Width).
		Height(m.vp.Height)
	if m.focus == focusTranscript {
		viewportStyle = viewportStyle.BorderForeground(colorInputBorder)
	}
	vpBox := viewportStyle.Render(m.vp.View())

	promptStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorPromptBorder).
		Width(max(1, m.width-2))
	if m.focus == focusPrompt || m.focus == focusSearch {
		promptStyle = promptStyle.BorderForeground(colorInputFocusBorder)
	}
	promptInner := lipgloss.NewStyle().Padding(0, 1).Render(m.ti.View())
	promptBorder := promptStyle.Render(promptInner)

	main := lipgloss.JoinVertical(lipgloss.Left, vpBox, status, promptBorder)

	if m.focus == focusSearch {
		hint := lipgloss.NewStyle().Foreground(colorInactive).Render("Enter keyword \u00B7 Esc cancel")
		searchInput := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorSearchBorder).
			Padding(0, 1).
			Width(min(60, m.width-4)).
			Render(m.searchTI.View())
		panel := lipgloss.JoinVertical(
			lipgloss.Left,
			hint,
			searchInput,
		)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, panel, lipgloss.WithWhitespaceChars(" "))
	}

	return main
}
