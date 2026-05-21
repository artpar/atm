package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const refreshInterval = 2 * time.Second

type viewMode int

const (
	viewTable viewMode = iota
	viewFeed
)

func (m viewMode) String() string {
	switch m {
	case viewFeed:
		return "feed"
	default:
		return "table"
	}
}

type agentsLoadedMsg struct {
	agents []AgentProcess
	err    error
	at     time.Time
}

type tickMsg time.Time

type tuiModel struct {
	table           table.Model
	filter          textinput.Model
	agents          []AgentProcess
	visible         []AgentProcess
	events          []AgentEvent
	visibleEvents   []AgentEvent
	sortMode        sortMode
	mode            viewMode
	width           int
	height          int
	filtering       bool
	showDetail      bool
	status          string
	lastRefresh     time.Time
	selectedID      string
	selectedEventID string
	feedCursor      int
	expandedEvents  map[string]bool
	refreshing      bool
	clipboardErr    string
}

func runTUI() error {
	model := newTUIModel()
	_, err := tea.NewProgram(model, tea.WithAltScreen()).Run()
	return err
}

func newTUIModel() tuiModel {
	input := textinput.New()
	input.Placeholder = "filter agents"
	input.Prompt = "/ "
	input.CharLimit = 128

	t := table.New(
		table.WithColumns(tableColumns(100)),
		table.WithRows(nil),
		table.WithFocused(true),
		table.WithHeight(12),
	)
	t.SetStyles(tableStyles())

	return tuiModel{
		table:          t,
		filter:         input,
		sortMode:       sortByActivity,
		mode:           viewTable,
		status:         "loading agents",
		expandedEvents: map[string]bool{},
	}
}

func (m tuiModel) Init() tea.Cmd {
	return tea.Batch(loadAgentsCmd(), tickCmd())
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeTable()
		return m, nil
	case agentsLoadedMsg:
		m.refreshing = false
		m.lastRefresh = msg.at
		if msg.err != nil {
			m.status = "refresh failed: " + msg.err.Error()
			return m, nil
		}
		m.agents = msg.agents
		m.status = fmt.Sprintf("updated %s", msg.at.Format("15:04:05"))
		m.rebuildViews()
		return m, nil
	case tickMsg:
		m.refreshing = true
		return m, tea.Batch(loadAgentsCmd(), tickCmd())
	case tea.KeyMsg:
		var cmd tea.Cmd
		if m.filtering {
			switch msg.String() {
			case "esc":
				m.filtering = false
				m.filter.Blur()
				return m, nil
			case "enter":
				m.filtering = false
				m.filter.Blur()
				return m, nil
			}
			m.filter, cmd = m.filter.Update(msg)
			m.rebuildViews()
			return m, cmd
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.toggleMode()
			return m, nil
		case "/":
			m.filtering = true
			m.filter.Focus()
			return m, textinput.Blink
		case "esc":
			if m.showDetail {
				m.showDetail = false
				m.resizeTable()
				return m, nil
			}
			if m.filter.Value() != "" {
				m.filter.SetValue("")
				m.rebuildViews()
				return m, nil
			}
		case "r":
			m.refreshing = true
			m.status = "refreshing"
			return m, loadAgentsCmd()
		case "s":
			if m.mode == viewTable {
				m.sortMode = (m.sortMode + 1) % 4
				m.status = "sort: " + m.sortMode.String()
				m.rebuildRows()
			}
			return m, nil
		case "enter":
			if m.mode == viewFeed {
				m.toggleSelectedEvent()
			} else {
				m.showDetail = !m.showDetail
				m.resizeTable()
			}
			return m, nil
		case "c":
			m.copySelected()
			return m, nil
		case "up", "k":
			if m.mode == viewFeed {
				m.moveFeedCursor(-1)
				return m, nil
			}
		case "down", "j":
			if m.mode == viewFeed {
				m.moveFeedCursor(1)
				return m, nil
			}
		}
	}

	if m.mode == viewFeed {
		return m, nil
	}
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	m.selectedID = selectedAgentID(m.visible, m.table.Cursor())
	return m, cmd
}

func (m tuiModel) View() string {
	if m.width == 0 {
		return "ATM loading\n"
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("ATM"))
	b.WriteString(" ")
	b.WriteString(m.summary())
	b.WriteString("\n")

	if m.filtering || m.filter.Value() != "" {
		b.WriteString(m.filter.View())
		b.WriteString("\n")
	}

	if m.mode == viewFeed {
		b.WriteString(m.feedView())
		b.WriteString("\n")
	} else {
		b.WriteString(m.table.View())
		b.WriteString("\n")

		if m.showDetail {
			b.WriteString(m.detailView())
			b.WriteString("\n")
		}
	}

	b.WriteString(footerStyle.Width(m.width).Render(m.helpText()))
	return b.String()
}

func (m *tuiModel) rebuildViews() {
	m.rebuildRows()
	m.rebuildEvents()
}

func (m *tuiModel) rebuildRows() {
	selected := m.selectedID
	if selected == "" {
		selected = selectedAgentID(m.visible, m.table.Cursor())
	}

	agents := append([]AgentProcess(nil), m.agents...)
	sortAgents(agents, m.sortMode)
	agents = filterAgents(agents, m.filter.Value())
	m.visible = agents

	rows := make([]table.Row, 0, len(agents))
	for _, agent := range agents {
		rows = append(rows, table.Row{
			healthCell(agent.Health),
			agent.Name,
			strconv.Itoa(agent.PID),
			agent.Project,
			agent.Elapsed,
			lastActivityLabel(agent.LastActivity, m.lastRefresh),
			agent.Activity,
		})
	}
	m.table.SetRows(rows)
	m.restoreSelection(selected)
}

func (m *tuiModel) rebuildEvents() {
	selected := m.selectedEventID
	if selected == "" {
		selected = selectedEventID(m.visibleEvents, m.feedCursor)
	}
	m.events = buildAgentEvents(m.agents, m.lastRefresh, 50)
	m.visibleEvents = filterEvents(m.events, m.filter.Value())
	m.restoreEventSelection(selected)
}

func (m *tuiModel) restoreSelection(id string) {
	if id == "" {
		m.selectedID = selectedAgentID(m.visible, m.table.Cursor())
		return
	}
	for i, agent := range m.visible {
		if agent.ID == id {
			m.table.SetCursor(i)
			m.selectedID = id
			return
		}
	}
	m.table.SetCursor(0)
	m.selectedID = selectedAgentID(m.visible, 0)
}

func (m *tuiModel) restoreEventSelection(id string) {
	if id == "" {
		m.selectedEventID = selectedEventID(m.visibleEvents, m.feedCursor)
		return
	}
	for i, event := range m.visibleEvents {
		if event.ID == id {
			m.feedCursor = i
			m.selectedEventID = id
			return
		}
	}
	if m.feedCursor >= len(m.visibleEvents) {
		m.feedCursor = len(m.visibleEvents) - 1
	}
	if m.feedCursor < 0 {
		m.feedCursor = 0
	}
	m.selectedEventID = selectedEventID(m.visibleEvents, m.feedCursor)
}

func (m *tuiModel) resizeTable() {
	m.table.SetColumns(tableColumns(m.width))
	detailHeight := 0
	if m.showDetail {
		detailHeight = 10
	}
	filterHeight := 0
	if m.filtering || m.filter.Value() != "" {
		filterHeight = 1
	}
	height := m.height - 4 - filterHeight - detailHeight
	if height < 5 {
		height = 5
	}
	m.table.SetHeight(height)
}

func (m *tuiModel) toggleMode() {
	if m.mode == viewTable {
		m.mode = viewFeed
		m.showDetail = false
		m.status = "view: feed"
		m.rebuildEvents()
		return
	}
	m.mode = viewTable
	m.status = "view: table"
	m.resizeTable()
}

func (m tuiModel) summary() string {
	active, idle, stale, unknown := 0, 0, 0, 0
	for _, agent := range m.agents {
		switch agent.Health {
		case healthActive:
			active++
		case healthIdle:
			idle++
		case healthStale:
			stale++
		default:
			unknown++
		}
	}
	parts := []string{
		fmt.Sprintf("%d agents", len(m.agents)),
		fmt.Sprintf("%s view", m.mode.String()),
		fmt.Sprintf("%d active", active),
		fmt.Sprintf("%d idle", idle),
		fmt.Sprintf("%d stale", stale),
	}
	if unknown > 0 {
		parts = append(parts, fmt.Sprintf("%d unknown", unknown))
	}
	if m.refreshing {
		parts = append(parts, "refreshing")
	}
	parts = append(parts, m.status)
	return mutedStyle.Render(strings.Join(parts, " | "))
}

func (m tuiModel) detailView() string {
	agent, ok := selectedAgent(m.visible, m.table.Cursor())
	if !ok {
		return detailStyle.Width(m.width).Render("No agent selected")
	}
	lines := []string{
		fmt.Sprintf("%s %s", agent.Name, mutedStyle.Render(agent.ID)),
		fmt.Sprintf("Status: %s | Source: %s | Last activity: %s", agent.Health, agent.Source, lastActivityLabel(agent.LastActivity, m.lastRefresh)),
		fmt.Sprintf("PID: %d | PPID: %d | Runtime: %s", agent.PID, agent.PPID, agent.Elapsed),
		"CWD: " + valueOrDash(agent.CWD),
		"Session: " + valueOrDash(agent.SessionID),
		"Session path: " + valueOrDash(agent.SessionPath),
		"Activity: " + valueOrDash(agent.Activity),
		"Command: " + valueOrDash(agent.Command),
	}
	if m.clipboardErr != "" {
		lines = append(lines, "Copy: "+m.clipboardErr)
	}
	return detailStyle.Width(m.width).Render(strings.Join(lines, "\n"))
}

func (m tuiModel) feedView() string {
	filterHeight := 0
	if m.filtering || m.filter.Value() != "" {
		filterHeight = 1
	}
	height := m.height - 4 - filterHeight
	if height < 5 {
		height = 5
	}
	if len(m.visibleEvents) == 0 {
		return mutedStyle.Render("No activity matches the current filter.")
	}

	start := m.feedCursor - height/3
	if start < 0 {
		start = 0
	}
	var b strings.Builder
	lines := 0
	for i := start; i < len(m.visibleEvents) && lines < height; i++ {
		event := m.visibleEvents[i]
		entry := m.feedEntry(event, i == m.feedCursor, height-lines)
		if entry == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n")
			lines++
		}
		b.WriteString(entry)
		lines += strings.Count(entry, "\n") + 1
	}
	return b.String()
}

func (m tuiModel) feedEntry(event AgentEvent, selected bool, remaining int) string {
	if remaining <= 0 {
		return ""
	}
	width := m.width
	if width <= 0 {
		width = 80
	}
	bodyWidth := width - 4
	if bodyWidth < 30 {
		bodyWidth = 30
	}

	marker := " "
	if selected {
		marker = ">"
	}
	header := fmt.Sprintf("%s %s %s %s/%d %s",
		marker,
		lastActivityLabel(event.Timestamp, m.lastRefresh),
		eventKindStyle(event.Kind).Render(event.Kind),
		valueOrDash(event.Project),
		event.PID,
		mutedStyle.Render(event.AgentName),
	)

	text := event.Text
	if text == "" {
		text = "process running"
	}
	expanded := m.expandedEvents[event.ID]
	maxLines := 2
	if expanded {
		maxLines = remaining - 1
		if maxLines < 2 {
			maxLines = 2
		}
		if maxLines > 8 {
			maxLines = 8
		}
	}
	wrapped := wrapText(text, bodyWidth)
	if len(wrapped) > maxLines {
		wrapped = wrapped[:maxLines]
		if len(wrapped) > 0 {
			wrapped[len(wrapped)-1] = wrapped[len(wrapped)-1] + " ..."
		}
	}

	lines := []string{header}
	for _, line := range wrapped {
		lines = append(lines, "  "+line)
	}
	if selected && m.clipboardErr != "" {
		lines = append(lines, "  copy: "+m.clipboardErr)
	}
	rendered := strings.Join(lines, "\n")
	if selected {
		return selectedFeedStyle.Width(width).Render(rendered)
	}
	return feedStyle.Width(width).Render(rendered)
}

func (m tuiModel) helpText() string {
	if m.filtering {
		return "enter apply | esc close filter | type to filter"
	}
	if m.mode == viewFeed {
		return "tab table | / filter | up/down select | enter expand | c copy event | r refresh | q quit"
	}
	return "tab feed | / filter | s sort " + m.sortMode.String() + " | r refresh | enter details | c copy | q quit"
}

func (m *tuiModel) copySelected() {
	if m.mode == viewFeed {
		event, ok := selectedEvent(m.visibleEvents, m.feedCursor)
		if !ok {
			m.status = "nothing selected"
			return
		}
		if event.Text == "" {
			m.status = "nothing to copy"
			return
		}
		if err := copyText(event.Text); err != nil {
			m.clipboardErr = event.Text
			m.status = "clipboard unavailable"
			m.expandedEvents[event.ID] = true
			return
		}
		m.clipboardErr = ""
		m.status = "copied event"
		return
	}

	agent, ok := selectedAgent(m.visible, m.table.Cursor())
	if !ok {
		m.status = "nothing selected"
		return
	}
	text := agent.SessionPath
	if text == "" {
		text = agent.Command
	}
	if text == "" {
		m.status = "nothing to copy"
		return
	}
	if err := copyText(text); err != nil {
		m.clipboardErr = text
		m.status = "clipboard unavailable; showing text in details"
		m.showDetail = true
		m.resizeTable()
		return
	}
	m.clipboardErr = ""
	m.status = "copied"
}

func (m *tuiModel) moveFeedCursor(delta int) {
	if len(m.visibleEvents) == 0 {
		m.feedCursor = 0
		m.selectedEventID = ""
		return
	}
	m.feedCursor += delta
	if m.feedCursor < 0 {
		m.feedCursor = 0
	}
	if m.feedCursor >= len(m.visibleEvents) {
		m.feedCursor = len(m.visibleEvents) - 1
	}
	m.selectedEventID = selectedEventID(m.visibleEvents, m.feedCursor)
}

func (m *tuiModel) toggleSelectedEvent() {
	event, ok := selectedEvent(m.visibleEvents, m.feedCursor)
	if !ok {
		return
	}
	m.expandedEvents[event.ID] = !m.expandedEvents[event.ID]
	m.selectedEventID = event.ID
}

func selectedAgent(agents []AgentProcess, cursor int) (AgentProcess, bool) {
	if cursor < 0 || cursor >= len(agents) {
		return AgentProcess{}, false
	}
	return agents[cursor], true
}

func selectedAgentID(agents []AgentProcess, cursor int) string {
	agent, ok := selectedAgent(agents, cursor)
	if !ok {
		return ""
	}
	return agent.ID
}

func selectedEvent(events []AgentEvent, cursor int) (AgentEvent, bool) {
	if cursor < 0 || cursor >= len(events) {
		return AgentEvent{}, false
	}
	return events[cursor], true
}

func selectedEventID(events []AgentEvent, cursor int) string {
	event, ok := selectedEvent(events, cursor)
	if !ok {
		return ""
	}
	return event.ID
}

func wrapText(text string, width int) []string {
	text = strings.Join(strings.Fields(text), " ")
	if text == "" {
		return nil
	}
	if width <= 0 {
		return []string{text}
	}
	words := strings.Fields(text)
	lines := make([]string, 0, len(text)/width+1)
	var current string
	for _, word := range words {
		if current == "" {
			current = hardWrapWord(word, width, &lines)
			continue
		}
		if len([]rune(current))+1+len([]rune(word)) <= width {
			current += " " + word
			continue
		}
		lines = append(lines, current)
		current = hardWrapWord(word, width, &lines)
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

func hardWrapWord(word string, width int, lines *[]string) string {
	runes := []rune(word)
	for len(runes) > width {
		*lines = append(*lines, string(runes[:width]))
		runes = runes[width:]
	}
	return string(runes)
}

func loadAgentsCmd() tea.Cmd {
	return func() tea.Msg {
		at := time.Now()
		agents, err := discover()
		return agentsLoadedMsg{agents: agents, err: err, at: at}
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(refreshInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func tableColumns(width int) []table.Column {
	if width <= 0 {
		width = 80
	}
	statusWidth := 7
	agentWidth := 8
	pidWidth := 6
	projectWidth := 14
	ageWidth := 11
	lastWidth := 6
	if width >= 110 {
		agentWidth = 12
		projectWidth = 22
		lastWidth = 7
	}
	if width >= 140 {
		projectWidth = 30
	}
	used := statusWidth + agentWidth + pidWidth + projectWidth + ageWidth + lastWidth + 14
	activityWidth := width - used
	if activityWidth < 14 {
		activityWidth = 14
	}
	return []table.Column{
		{Title: "Status", Width: statusWidth},
		{Title: "Agent", Width: agentWidth},
		{Title: "PID", Width: pidWidth},
		{Title: "Project", Width: projectWidth},
		{Title: "Age", Width: ageWidth},
		{Title: "Last", Width: lastWidth},
		{Title: "Current Activity", Width: activityWidth},
	}
}

func tableStyles() table.Styles {
	styles := table.DefaultStyles()
	styles.Header = styles.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	styles.Selected = styles.Selected.
		Foreground(lipgloss.Color("230")).
		Background(lipgloss.Color("238")).
		Bold(false)
	return styles
}

func copyText(text string) error {
	cmd := exec.Command("pbcopy")
	cmd.Stdin = bytes.NewBufferString(text)
	return cmd.Run()
}

func healthCell(health string) string {
	switch health {
	case healthActive:
		return activeStyle.Render(health)
	case healthIdle:
		return idleStyle.Render(health)
	case healthStale:
		return staleStyle.Render(health)
	default:
		return unknownStyle.Render(health)
	}
}

func eventKindStyle(kind string) lipgloss.Style {
	switch kind {
	case "user":
		return userEventStyle
	case "assistant":
		return assistantEventStyle
	case "tool":
		return toolEventStyle
	default:
		return statusEventStyle
	}
}

func valueOrDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	mutedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	footerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).PaddingTop(1)
	activeStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	idleStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	staleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	unknownStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))
	userEventStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))
	assistantEventStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("120"))
	toolEventStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("213"))
	statusEventStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	feedStyle           = lipgloss.NewStyle().PaddingLeft(1)
	selectedFeedStyle   = lipgloss.NewStyle().Background(lipgloss.Color("238")).PaddingLeft(1)
	detailStyle         = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("240")).
				Padding(0, 1)
)
