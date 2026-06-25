package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

type AgentProcess struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	PID          int       `json:"pid"`
	PPID         int       `json:"ppid"`
	Elapsed      string    `json:"elapsed"`
	Usage        Usage     `json:"usage"`
	CWD          string    `json:"cwd,omitempty"`
	Project      string    `json:"project,omitempty"`
	Command      string    `json:"command"`
	Status       string    `json:"status"`
	Health       string    `json:"health,omitempty"`
	Source       string    `json:"source,omitempty"`
	Activity     string    `json:"activity,omitempty"`
	SessionID    string    `json:"session_id,omitempty"`
	SessionPath  string    `json:"session_path,omitempty"`
	LastActivity time.Time `json:"last_activity,omitempty"`
}

type psProcess struct {
	PID        int
	PPID       int
	Elapsed    string
	CPUPercent float64
	RSSBytes   uint64
	Command    string
}

type codexSession struct {
	ID       string
	CWD      string
	Path     string
	ModTime  time.Time
	Activity string
}

type detector struct {
	agents map[string]string
}

func newDetector() detector {
	return detector{agents: map[string]string{
		"codex":        "codex",
		"claude":       "claude",
		"gemini":       "gemini",
		"aider":        "aider",
		"opencode":     "opencode",
		"goose":        "goose",
		"amp":          "amp",
		"cursor-agent": "cursor-agent",
	}}
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "atm:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return runTUI()
	}

	switch args[0] {
	case "tui":
		return runTUI()
	case "list", "ls":
		return runList(args[1:], stdout)
	case "inspect":
		return runInspect(args[1:], stdout)
	case "version":
		fmt.Fprintf(stdout, "atm %s (%s, %s)\n", displayVersion(), commit, date)
		return nil
	case "help", "-h", "--help":
		printUsage(stdout)
		return nil
	default:
		return fmt.Errorf("unknown command %q\n\nRun `atm help` for usage.", args[0])
	}
}

func displayVersion() string {
	if version != "dev" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return version
	}
	return info.Main.Version
}

func runList(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonOut := fs.Bool("json", false, "print JSON")
	watch := fs.Duration("watch", 0, "refresh interval, for example 2s")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *watch > 0 {
		for {
			agents, err := discover()
			if err != nil {
				return err
			}
			fmt.Fprint(stdout, "\033[H\033[2J")
			if err := printAgents(stdout, agents, *jsonOut); err != nil {
				return err
			}
			time.Sleep(*watch)
		}
	}

	agents, err := discover()
	if err != nil {
		return err
	}
	return printAgents(stdout, agents, *jsonOut)
}

func runInspect(args []string, stdout io.Writer) error {
	jsonOut, needle, err := parseInspectArgs(args)
	if err != nil {
		return err
	}

	agents, err := discover()
	if err != nil {
		return err
	}
	for _, agent := range agents {
		if strconv.Itoa(agent.PID) == needle || agent.ID == needle {
			if jsonOut {
				enc := json.NewEncoder(stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(agent)
			}
			return printAgentDetail(stdout, agent)
		}
	}
	return fmt.Errorf("no agent process matched %q", needle)
}

func parseInspectArgs(args []string) (bool, string, error) {
	var jsonOut bool
	var positional []string
	for _, arg := range args {
		switch arg {
		case "-json", "--json":
			jsonOut = true
		default:
			if strings.HasPrefix(arg, "-") {
				return false, "", fmt.Errorf("unknown inspect flag %q", arg)
			}
			positional = append(positional, arg)
		}
	}
	if len(positional) != 1 {
		return false, "", errors.New("usage: atm inspect <pid|agent-id> [-json]")
	}
	return jsonOut, positional[0], nil
}

func discover() ([]AgentProcess, error) {
	processes, err := ps()
	if err != nil {
		return nil, err
	}

	d := newDetector()
	var agents []AgentProcess
	codexCWDs := map[string]bool{}
	for _, proc := range processes {
		name, ok := d.detect(proc.Command)
		if !ok {
			continue
		}
		cwd := cwdForPID(proc.PID)
		agent := AgentProcess{
			ID:      fmt.Sprintf("%s:%d", name, proc.PID),
			Name:    name,
			PID:     proc.PID,
			PPID:    proc.PPID,
			Elapsed: proc.Elapsed,
			Usage: Usage{
				CPUPercent: proc.CPUPercent,
				RSSBytes:   proc.RSSBytes,
			},
			CWD:     cwd,
			Command: proc.Command,
			Status:  "running",
		}
		enrichUsage(&agent)
		if name == "codex" {
			codexCWDs[cwd] = true
		}
		agents = append(agents, agent)
	}

	codexSessions := recentCodexSessions(200, codexCWDs)
	for i := range agents {
		if agents[i].Name == "codex" {
			enrichCodex(&agents[i], codexSessions)
		}
	}
	decorateAgents(agents, time.Now())

	sort.Slice(agents, func(i, j int) bool {
		if agents[i].Name != agents[j].Name {
			return agents[i].Name < agents[j].Name
		}
		return agents[i].PID < agents[j].PID
	})
	return agents, nil
}

func (d detector) detect(command string) (string, bool) {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return "", false
	}

	base := strings.ToLower(filepath.Base(fields[0]))
	base = strings.TrimSuffix(base, ".exe")
	if name, ok := d.agents[base]; ok {
		return name, true
	}

	if !isScriptLauncher(base) {
		if name, ok := d.detectShellScript(base, fields); ok {
			return name, true
		}
		return "", false
	}

	// Node-based CLIs often appear as `node /path/to/<agent>/bin.js`.
	for _, field := range fields[1:] {
		clean := strings.ToLower(filepath.Base(field))
		clean = strings.TrimSuffix(clean, ".js")
		clean = strings.TrimSuffix(clean, ".mjs")
		if name, ok := d.agents[clean]; ok {
			return name, true
		}
	}
	return "", false
}

func (d detector) detectShellScript(base string, fields []string) (string, bool) {
	if !isShellLauncher(base) || len(fields) < 2 || strings.HasPrefix(fields[1], "-") {
		return "", false
	}
	clean := strings.ToLower(filepath.Base(fields[1]))
	clean = strings.TrimSuffix(clean, ".sh")
	name, ok := d.agents[clean]
	return name, ok
}

func isScriptLauncher(base string) bool {
	switch base {
	case "node", "bun", "deno":
		return true
	default:
		return false
	}
}

func isShellLauncher(base string) bool {
	switch base {
	case "sh", "bash", "zsh":
		return true
	default:
		return false
	}
}

func ps() ([]psProcess, error) {
	out, err := exec.Command("ps", "-axo", "pid=,ppid=,etime=,pcpu=,rss=,command=").Output()
	if err != nil {
		return nil, err
	}

	var processes []psProcess
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		proc, ok := parsePSLine(line)
		if ok {
			processes = append(processes, proc)
		}
	}
	return processes, scanner.Err()
}

func parsePSLine(line string) (psProcess, bool) {
	parts := strings.Fields(line)
	if len(parts) < 6 {
		return psProcess{}, false
	}
	pid, err := strconv.Atoi(parts[0])
	if err != nil {
		return psProcess{}, false
	}
	ppid, err := strconv.Atoi(parts[1])
	if err != nil {
		return psProcess{}, false
	}
	cpuPercent, err := strconv.ParseFloat(parts[3], 64)
	if err != nil {
		return psProcess{}, false
	}
	rssKB, err := strconv.ParseUint(parts[4], 10, 64)
	if err != nil {
		return psProcess{}, false
	}

	rest := strings.TrimSpace(line)
	for i := 0; i < 5; i++ {
		idx := strings.IndexFunc(rest, func(r rune) bool { return r == ' ' || r == '\t' })
		if idx < 0 {
			return psProcess{}, false
		}
		rest = strings.TrimSpace(rest[idx:])
	}

	return psProcess{
		PID:        pid,
		PPID:       ppid,
		Elapsed:    parts[2],
		CPUPercent: cpuPercent,
		RSSBytes:   rssKB * 1024,
		Command:    rest,
	}, true
}

func recentCodexSessions(limit int, targetCWDs map[string]bool) []codexSession {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	root := filepath.Join(home, ".codex", "sessions")

	var sessions []codexSession
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".jsonl") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		sessions = append(sessions, codexSession{Path: path, ModTime: info.ModTime()})
		return nil
	})

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].ModTime.After(sessions[j].ModTime)
	})
	if len(sessions) > limit {
		sessions = sessions[:limit]
	}

	filtered := sessions[:0]
	for _, session := range sessions {
		loadCodexSessionMeta(&session)
		if len(targetCWDs) > 0 && !targetCWDs[session.CWD] {
			continue
		}
		loadCodexSessionActivity(&session)
		filtered = append(filtered, session)
	}
	return filtered
}

func loadCodexSessionMeta(session *codexSession) {
	file, err := os.Open(session.Path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	if scanner.Scan() {
		readCodexMeta(scanner.Text(), session)
	}
}

func loadCodexSessionActivity(session *codexSession) {
	file, err := os.Open(session.Path)
	if err != nil {
		return
	}
	defer file.Close()

	const tailBytes int64 = 512 * 1024
	info, err := file.Stat()
	if err != nil {
		return
	}

	size := info.Size()
	readSize := tailBytes
	if size < readSize {
		readSize = size
	}
	offset := size - readSize
	buf := make([]byte, readSize)
	n, err := file.ReadAt(buf, offset)
	if err != nil && err != io.EOF {
		return
	}
	lines := strings.Split(string(buf[:n]), "\n")
	start := 0
	if offset > 0 {
		start = 1
	}
	for i := len(lines) - 1; i >= start; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if activity := summarizeCodexLine(line); activity != "" {
			session.Activity = activity
			break
		}
	}
}

func readCodexMeta(line string, session *codexSession) {
	var row struct {
		Type    string `json:"type"`
		Payload struct {
			ID  string `json:"id"`
			CWD string `json:"cwd"`
		} `json:"payload"`
	}
	if json.Unmarshal([]byte(line), &row) != nil || row.Type != "session_meta" {
		return
	}
	session.ID = row.Payload.ID
	session.CWD = row.Payload.CWD
}

func summarizeCodexLine(line string) string {
	if strings.Contains(line, `"type":"reasoning"`) || strings.Contains(line, `"encrypted_content"`) {
		return ""
	}
	var row struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}
	if json.Unmarshal([]byte(line), &row) != nil {
		return ""
	}

	switch row.Type {
	case "event_msg":
		var event struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		}
		if json.Unmarshal(row.Payload, &event) == nil && event.Message != "" {
			return compact("user: " + event.Message)
		}
	case "response_item":
		var item struct {
			Type    string `json:"type"`
			Name    string `json:"name"`
			Role    string `json:"role"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		}
		if json.Unmarshal(row.Payload, &item) != nil {
			return ""
		}
		if item.Type == "function_call" && item.Name != "" {
			return compact("tool: " + item.Name)
		}
		if item.Type == "message" {
			for _, part := range item.Content {
				if part.Text != "" {
					role := item.Role
					if role == "" {
						role = "message"
					}
					return compact(role + ": " + part.Text)
				}
			}
		}
	}
	return ""
}

func enrichCodex(agent *AgentProcess, sessions []codexSession) {
	var best *codexSession
	for i := range sessions {
		session := &sessions[i]
		if agent.CWD != "" && session.CWD != agent.CWD {
			continue
		}
		if best == nil || session.ModTime.After(best.ModTime) {
			best = session
		}
	}
	if best == nil && len(sessions) > 0 {
		best = &sessions[0]
	}
	if best == nil {
		return
	}
	agent.SessionID = best.ID
	agent.SessionPath = best.Path
	agent.LastActivity = best.ModTime
	agent.Activity = best.Activity
}

func printAgents(w io.Writer, agents []AgentProcess, jsonOut bool) error {
	if jsonOut {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(agents)
	}
	if len(agents) == 0 {
		fmt.Fprintln(w, "No known agent processes are running.")
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "AGENT\tPID\tCPU\tMEM\tDISK\tNET\tELAPSED\tCWD\tACTIVITY")
	for _, agent := range agents {
		fmt.Fprintf(tw, "%s\t%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			agent.Name,
			agent.PID,
			formatCPU(agent.Usage.CPUPercent),
			formatBytes(agent.Usage.RSSBytes),
			formatDisk(agent.Usage),
			formatNetwork(agent.Usage),
			agent.Elapsed,
			shortPath(agent.CWD),
			agent.Activity,
		)
	}
	return tw.Flush()
}

func printAgentDetail(w io.Writer, agent AgentProcess) error {
	fmt.Fprintf(w, "Agent: %s\n", agent.Name)
	fmt.Fprintf(w, "PID: %d\n", agent.PID)
	fmt.Fprintf(w, "PPID: %d\n", agent.PPID)
	fmt.Fprintf(w, "Status: %s\n", agent.Status)
	fmt.Fprintf(w, "Elapsed: %s\n", agent.Elapsed)
	fmt.Fprintf(w, "CPU: %s\n", formatCPU(agent.Usage.CPUPercent))
	fmt.Fprintf(w, "Memory: %s\n", formatBytes(agent.Usage.RSSBytes))
	fmt.Fprintf(w, "Disk I/O: %s\n", formatDisk(agent.Usage))
	fmt.Fprintf(w, "Network: %s\n", formatNetwork(agent.Usage))
	if agent.CWD != "" {
		fmt.Fprintf(w, "CWD: %s\n", agent.CWD)
	}
	if agent.Activity != "" {
		fmt.Fprintf(w, "Activity: %s\n", agent.Activity)
	}
	if agent.SessionID != "" {
		fmt.Fprintf(w, "Session: %s\n", agent.SessionID)
	}
	if !agent.LastActivity.IsZero() {
		fmt.Fprintf(w, "Last activity: %s\n", agent.LastActivity.Format(time.RFC3339))
	}
	if agent.SessionPath != "" {
		fmt.Fprintf(w, "Session path: %s\n", agent.SessionPath)
	}
	fmt.Fprintf(w, "Command: %s\n", agent.Command)
	return nil
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `ATM - Agent task manager

Usage:
  atm
  atm tui
  atm list [-json] [-watch 2s]
  atm inspect <pid|agent-id> [-json]
  atm version

Known agents:
  codex, claude, gemini, aider, opencode, goose, amp, cursor-agent

`)
}

func compact(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	const max = 120
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max-3]) + "..."
}

func shortPath(path string) string {
	if path == "" {
		return "-"
	}
	home, err := os.UserHomeDir()
	if err == nil && strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}
