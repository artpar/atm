//go:build !darwin && !linux

package main

func cwdForPID(pid int) string {
	return ""
}
