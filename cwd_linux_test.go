//go:build linux

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLinuxProcCWD(t *testing.T) {
	root := t.TempDir()
	target := t.TempDir()
	pidDir := filepath.Join(root, "123")
	if err := os.MkdirAll(pidDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(pidDir, "cwd")); err != nil {
		t.Fatal(err)
	}

	got, err := linuxProcCWD(root, 123)
	if err != nil {
		t.Fatal(err)
	}
	if got != target {
		t.Fatalf("linuxProcCWD = %q, want %q", got, target)
	}
}
