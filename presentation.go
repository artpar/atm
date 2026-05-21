package main

import (
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	healthActive  = "active"
	healthIdle    = "idle"
	healthStale   = "stale"
	healthUnknown = "unknown"
)

type sortMode int

const (
	sortByActivity sortMode = iota
	sortByAgent
	sortByProject
	sortByPID
)

func (m sortMode) String() string {
	switch m {
	case sortByAgent:
		return "agent"
	case sortByProject:
		return "project"
	case sortByPID:
		return "pid"
	default:
		return "activity"
	}
}

func decorateAgents(agents []AgentProcess, now time.Time) {
	for i := range agents {
		agents[i].Project = projectName(agents[i].CWD)
		agents[i].Health = classifyHealth(agents[i].LastActivity, now)
		agents[i].Source = sourceForAgent(agents[i])
		if agents[i].Activity == "" {
			agents[i].Activity = "process running"
		}
	}
}

func projectName(cwd string) string {
	if cwd == "" {
		return "-"
	}
	clean := filepath.Clean(cwd)
	base := filepath.Base(clean)
	if base == "." || base == string(filepath.Separator) {
		return clean
	}
	return base
}

func sourceForAgent(agent AgentProcess) string {
	if agent.SessionID != "" {
		return "codex session"
	}
	return "process only"
}

func classifyHealth(lastActivity time.Time, now time.Time) string {
	if lastActivity.IsZero() {
		return healthUnknown
	}
	age := now.Sub(lastActivity)
	if age < 0 {
		age = 0
	}
	switch {
	case age <= 2*time.Minute:
		return healthActive
	case age <= 15*time.Minute:
		return healthIdle
	default:
		return healthStale
	}
}

func lastActivityLabel(lastActivity time.Time, now time.Time) string {
	if lastActivity.IsZero() {
		return "-"
	}
	age := now.Sub(lastActivity)
	if age < 0 {
		age = 0
	}
	switch {
	case age < 5*time.Second:
		return "now"
	case age < time.Minute:
		return strconv.Itoa(int(age.Seconds())) + "s"
	case age < time.Hour:
		return strconv.Itoa(int(age.Minutes())) + "m"
	case age < 24*time.Hour:
		return strconv.Itoa(int(age.Hours())) + "h"
	default:
		return strconv.Itoa(int(age.Hours()/24)) + "d"
	}
}

func sortAgents(agents []AgentProcess, mode sortMode) {
	sort.SliceStable(agents, func(i, j int) bool {
		a := agents[i]
		b := agents[j]
		switch mode {
		case sortByAgent:
			if a.Name != b.Name {
				return a.Name < b.Name
			}
		case sortByProject:
			if a.Project != b.Project {
				return a.Project < b.Project
			}
		case sortByPID:
			return a.PID < b.PID
		default:
			if a.LastActivity.IsZero() != b.LastActivity.IsZero() {
				return !a.LastActivity.IsZero()
			}
			if !a.LastActivity.Equal(b.LastActivity) {
				return a.LastActivity.After(b.LastActivity)
			}
		}
		if a.Project != b.Project {
			return a.Project < b.Project
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.PID < b.PID
	})
}

func filterAgents(agents []AgentProcess, query string) []AgentProcess {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return agents
	}
	filtered := make([]AgentProcess, 0, len(agents))
	for _, agent := range agents {
		if agentMatches(agent, query) {
			filtered = append(filtered, agent)
		}
	}
	return filtered
}

func agentMatches(agent AgentProcess, query string) bool {
	values := []string{
		agent.ID,
		agent.Name,
		strconv.Itoa(agent.PID),
		agent.CWD,
		agent.Project,
		agent.Activity,
		agent.SessionID,
		agent.Health,
		agent.Source,
	}
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), query) {
			return true
		}
	}
	return false
}
