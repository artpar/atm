package main

import "testing"

func TestParsePSLine(t *testing.T) {
	proc, ok := parsePSLine("60204 74009       02:00 codex --dangerously-bypass-approvals-and-sandbox")
	if !ok {
		t.Fatal("expected ps line to parse")
	}
	if proc.PID != 60204 || proc.PPID != 74009 {
		t.Fatalf("unexpected pids: %#v", proc)
	}
	if proc.Elapsed != "02:00" {
		t.Fatalf("unexpected elapsed: %q", proc.Elapsed)
	}
	if proc.Command != "codex --dangerously-bypass-approvals-and-sandbox" {
		t.Fatalf("unexpected command: %q", proc.Command)
	}
}

func TestDetectKnownAgents(t *testing.T) {
	d := newDetector()
	tests := map[string]string{
		"codex --dangerously-bypass-approvals-and-sandbox": "codex",
		"node /opt/homebrew/bin/aider.js":                  "aider",
		"/usr/local/bin/opencode run":                      "opencode",
	}
	for command, want := range tests {
		got, ok := d.detect(command)
		if !ok {
			t.Fatalf("expected %q to be detected", command)
		}
		if got != want {
			t.Fatalf("detect(%q) = %q, want %q", command, got, want)
		}
	}
}

func TestSummarizeCodexLine(t *testing.T) {
	line := `{"type":"response_item","payload":{"type":"function_call","name":"exec_command"}}`
	if got := summarizeCodexLine(line); got != "tool: exec_command" {
		t.Fatalf("unexpected summary: %q", got)
	}
}

func TestParseInspectArgsAcceptsJSONAnywhere(t *testing.T) {
	jsonOut, needle, err := parseInspectArgs([]string{"61047", "-json"})
	if err != nil {
		t.Fatal(err)
	}
	if !jsonOut || needle != "61047" {
		t.Fatalf("unexpected parse result: json=%v needle=%q", jsonOut, needle)
	}
}

func TestDisplayVersionUsesInjectedVersion(t *testing.T) {
	oldVersion := version
	version = "v9.9.9"
	t.Cleanup(func() { version = oldVersion })

	if got := displayVersion(); got != "v9.9.9" {
		t.Fatalf("displayVersion = %q, want v9.9.9", got)
	}
}
