package main

import (
	"testing"
	"time"
)

func TestClassifyHealth(t *testing.T) {
	now := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		at   time.Time
		want string
	}{
		{name: "unknown", want: healthUnknown},
		{name: "active", at: now.Add(-90 * time.Second), want: healthActive},
		{name: "idle", at: now.Add(-10 * time.Minute), want: healthIdle},
		{name: "stale", at: now.Add(-20 * time.Minute), want: healthStale},
	}
	for _, test := range tests {
		if got := classifyHealth(test.at, now); got != test.want {
			t.Fatalf("%s: classifyHealth = %q, want %q", test.name, got, test.want)
		}
	}
}

func TestProjectName(t *testing.T) {
	if got := projectName("/Users/artpar/workspace/code/atm"); got != "atm" {
		t.Fatalf("projectName = %q, want atm", got)
	}
	if got := projectName(""); got != "-" {
		t.Fatalf("empty projectName = %q, want -", got)
	}
}

func TestSortAgentsByActivity(t *testing.T) {
	now := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)
	agents := []AgentProcess{
		{ID: "old", PID: 2, LastActivity: now.Add(-10 * time.Minute), Project: "b"},
		{ID: "unknown", PID: 3, Project: "c"},
		{ID: "new", PID: 1, LastActivity: now.Add(-1 * time.Minute), Project: "a"},
	}
	sortAgents(agents, sortByActivity)
	if agents[0].ID != "new" || agents[1].ID != "old" || agents[2].ID != "unknown" {
		t.Fatalf("unexpected order: %#v", agents)
	}
}

func TestFilterAgents(t *testing.T) {
	agents := []AgentProcess{
		{ID: "codex:1", Name: "codex", PID: 1, CWD: "/tmp/atm", Project: "atm", Activity: "tool: exec_command", Usage: Usage{CPUPercent: 4.2}},
		{ID: "aider:2", Name: "aider", PID: 2, CWD: "/tmp/other", Project: "other", Activity: "process running"},
	}
	if got := filterAgents(agents, "exec"); len(got) != 1 || got[0].ID != "codex:1" {
		t.Fatalf("filter by activity failed: %#v", got)
	}
	if got := filterAgents(agents, "aider:2"); len(got) != 1 || got[0].ID != "aider:2" {
		t.Fatalf("filter by pid failed: %#v", got)
	}
	if got := filterAgents(agents, "4.2%"); len(got) != 1 || got[0].ID != "codex:1" {
		t.Fatalf("filter by cpu failed: %#v", got)
	}
}
