//go:build linux

package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func enrichUsage(agent *AgentProcess) {
	readBytes, writeBytes, ok := linuxProcIO("/proc", agent.PID)
	if ok {
		agent.Usage.DiskReadBytes = readBytes
		agent.Usage.DiskWriteBytes = writeBytes
		agent.Usage.DiskAvailable = true
	}
	connections, ok := linuxNetworkConnections("/proc", agent.PID)
	if ok {
		agent.Usage.NetworkConnections = connections
		agent.Usage.NetworkAvailable = true
	}
}

func linuxProcIO(procRoot string, pid int) (uint64, uint64, bool) {
	file, err := os.Open(filepath.Join(procRoot, strconv.Itoa(pid), "io"))
	if err != nil {
		return 0, 0, false
	}
	defer file.Close()

	var readBytes, writeBytes uint64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 {
			continue
		}
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		switch strings.TrimSuffix(fields[0], ":") {
		case "read_bytes":
			readBytes = value
		case "write_bytes":
			writeBytes = value
		}
	}
	return readBytes, writeBytes, scanner.Err() == nil
}

func linuxNetworkConnections(procRoot string, pid int) (int, bool) {
	socketInodes, ok := linuxSocketInodes(procRoot, pid)
	if !ok {
		return 0, false
	}
	if len(socketInodes) == 0 {
		return 0, true
	}

	networkInodes := map[string]bool{}
	for _, name := range []string{"tcp", "tcp6", "udp", "udp6"} {
		for inode := range linuxProcNetInodes(procRoot, name) {
			networkInodes[inode] = true
		}
	}

	count := 0
	for inode := range socketInodes {
		if networkInodes[inode] {
			count++
		}
	}
	return count, true
}

func linuxSocketInodes(procRoot string, pid int) (map[string]bool, bool) {
	fdDir := filepath.Join(procRoot, strconv.Itoa(pid), "fd")
	entries, err := os.ReadDir(fdDir)
	if err != nil {
		return nil, false
	}
	inodes := map[string]bool{}
	for _, entry := range entries {
		target, err := os.Readlink(filepath.Join(fdDir, entry.Name()))
		if err != nil {
			continue
		}
		if strings.HasPrefix(target, "socket:[") && strings.HasSuffix(target, "]") {
			inodes[strings.TrimSuffix(strings.TrimPrefix(target, "socket:["), "]")] = true
		}
	}
	return inodes, true
}

func linuxProcNetInodes(procRoot, name string) map[string]bool {
	file, err := os.Open(filepath.Join(procRoot, "net", name))
	if err != nil {
		return nil
	}
	defer file.Close()

	inodes := map[string]bool{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 10 && fields[9] != "inode" {
			inodes[fields[9]] = true
		}
	}
	return inodes
}
