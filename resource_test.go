package main

import "testing"

func TestFormatUsage(t *testing.T) {
	usage := Usage{
		DiskReadBytes:      1024,
		DiskWriteBytes:     1024,
		DiskAvailable:      true,
		NetworkConnections: 1,
		NetworkAvailable:   true,
	}
	if got := formatCPU(12.25); got != "12.2%" {
		t.Fatalf("formatCPU = %q, want 12.2%%", got)
	}
	if got := formatBytes(1536); got != "1.5KiB" {
		t.Fatalf("formatBytes = %q, want 1.5KiB", got)
	}
	if got := formatDisk(usage); got != "2.0KiB" {
		t.Fatalf("formatDisk = %q, want 2.0KiB", got)
	}
	if got := formatNetwork(usage); got != "1 conn" {
		t.Fatalf("formatNetwork = %q, want 1 conn", got)
	}
	if got := formatDisk(Usage{}); got != "-" {
		t.Fatalf("unavailable disk = %q, want -", got)
	}
	if got := formatNetwork(Usage{}); got != "-" {
		t.Fatalf("unavailable network = %q, want -", got)
	}
}
