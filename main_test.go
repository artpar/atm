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

func TestDetectDoesNotMatchShellArguments(t *testing.T) {
	d := newDetector()
	command := `sh -lc "cd /tmp/work && codex &"`
	if got, ok := d.detect(command); ok {
		t.Fatalf("detect(%q) = %q, want no match", command, got)
	}
}

func TestDetectShellScriptAgent(t *testing.T) {
	d := newDetector()
	command := "/bin/sh /tmp/atm-smoke/bin/codex"
	got, ok := d.detect(command)
	if !ok {
		t.Fatalf("expected shell script agent to be detected")
	}
	if got != "codex" {
		t.Fatalf("detect(%q) = %q, want codex", command, got)
	}
}

func TestSummarizeCodexLine(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{
			name: "user",
			line: `{"timestamp":"2026-05-21T04:00:00Z","type":"event_msg","payload":{"type":"user_message","message":"what changed?"}}`,
			want: "user: what changed?",
		},
		{
			name: "tool",
			line: `{"timestamp":"2026-05-21T04:00:01Z","type":"response_item","payload":{"type":"function_call","name":"exec_command"}}`,
			want: "tool: exec_command",
		},
		{
			name: "assistant",
			line: `{"timestamp":"2026-05-21T04:00:02Z","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"done"}]}}`,
			want: "assistant: done",
		},
	}
	for _, test := range tests {
		if got := summarizeCodexLine(test.line); got != test.want {
			t.Fatalf("%s: summarizeCodexLine = %q, want %q", test.name, got, test.want)
		}
	}
}

func TestSummarizeCodexLineIgnoresReasoning(t *testing.T) {
	line := `{"type":"response_item","payload":{"type":"reasoning","encrypted_content":"secret"}}`
	if got := summarizeCodexLine(line); got != "" {
		t.Fatalf("expected no summary, got %q", got)
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
