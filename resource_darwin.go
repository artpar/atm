//go:build darwin

package main

import (
	"bufio"
	"errors"
	"os/exec"
	"strconv"
	"strings"
)

func enrichUsage(agent *AgentProcess) {
	out, err := exec.Command("lsof", "-a", "-p", strconv.Itoa(agent.PID), "-i", "-n", "-P").Output()
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		return
	}
	agent.Usage.NetworkConnections = countLsofNetworkRows(string(out))
	agent.Usage.NetworkAvailable = true
}

func countLsofNetworkRows(output string) int {
	count := 0
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "COMMAND ") {
			continue
		}
		count++
	}
	return count
}
