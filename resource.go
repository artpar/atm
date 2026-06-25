package main

import "fmt"

type Usage struct {
	CPUPercent         float64 `json:"cpu_percent"`
	RSSBytes           uint64  `json:"rss_bytes,omitempty"`
	DiskReadBytes      uint64  `json:"disk_read_bytes,omitempty"`
	DiskWriteBytes     uint64  `json:"disk_write_bytes,omitempty"`
	DiskAvailable      bool    `json:"disk_available"`
	NetworkConnections int     `json:"network_connections,omitempty"`
	NetworkAvailable   bool    `json:"network_available"`
}

func formatCPU(cpu float64) string {
	return fmt.Sprintf("%.1f%%", cpu)
}

func formatDisk(usage Usage) string {
	if !usage.DiskAvailable {
		return "-"
	}
	total := usage.DiskReadBytes + usage.DiskWriteBytes
	if total == 0 {
		return "0B"
	}
	return formatBytes(total)
}

func formatNetwork(usage Usage) string {
	if !usage.NetworkAvailable {
		return "-"
	}
	if usage.NetworkConnections == 1 {
		return "1 conn"
	}
	return fmt.Sprintf("%d conns", usage.NetworkConnections)
}

func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%dB", bytes)
	}
	value := float64(bytes)
	for _, suffix := range []string{"KiB", "MiB", "GiB", "TiB"} {
		value /= unit
		if value < unit {
			return fmt.Sprintf("%.1f%s", value, suffix)
		}
	}
	return fmt.Sprintf("%.1fPiB", value/unit)
}
