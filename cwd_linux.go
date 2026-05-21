//go:build linux

package main

import (
	"os"
	"path/filepath"
	"strconv"
)

func cwdForPID(pid int) string {
	cwd, err := linuxProcCWD("/proc", pid)
	if err != nil {
		return ""
	}
	return cwd
}

func linuxProcCWD(procRoot string, pid int) (string, error) {
	return os.Readlink(filepath.Join(procRoot, strconv.Itoa(pid), "cwd"))
}
