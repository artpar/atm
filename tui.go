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

type agentsLoadedMsg struct {
	agents []AgentProcess
	err    error
	at     time.Time
}

type tickMsg time.Time

type tuiModel struct {
	table        table.Model
	filter       textinput.Model
	agents       []AgentProcess
	visible      []AgentProcess
	sortMode     sortMode
	width        int
	height       int
	filtering    bool
	showDetail   bool
	status       string
	lastRefresh  time.Time
	selectedID   string
	refreshing   bool
	clipboardErr string
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
		table:    t,
		filter:   input,
		sortMode: sortByActivity,
		status:   "loading agents",
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
		m.rebuildRows()
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
			m.rebuildRows()
			return m, cmd
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
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
				m.rebuildRows()
				return m, nil
			}
		case "r":
			m.refreshing = true
			m.status = "refreshing"
			return m, loadAgentsCmd()
		case "s":
			m.sortMode = (m.sortMode + 1) % 4
			m.status = "sort: " + m.sortMode.String()
			m.rebuildRows()
			return m, nil
		case "enter":
			m.showDetail = !m.showDetail
			m.resizeTable()
			return m, nil
		case "c":
			m.copySelected()
			return m, nil
		}
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

	b.WriteString(m.table.View())
	b.WriteString("\n")

	if m.showDetail {
		b.WriteString(m.detailView())
		b.WriteString("\n")
	}

	b.WriteString(footerStyle.Width(m.width).Render(m.helpText()))
	return b.String()
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

func (m tuiModel) helpText() string {
	if m.filtering {
		return "enter apply | esc close filter | type to filter"
	}
	return "/ filter | s sort " + m.sortMode.String() + " | r refresh | enter details | c copy | q quit"
}

func (m *tuiModel) copySelected() {
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
	detailStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1)
)
