package system

import (
	"os"
	"runtime"
	"time"

	cpu "github.com/shirou/gopsutil/v4/cpu"
	disk "github.com/shirou/gopsutil/v4/disk"
	memory "github.com/shirou/gopsutil/v4/mem"
)

type Snapshot struct {
	CPUPercent float64
	MemUsed    uint64
	MemTotal   uint64
	DiskUsed   uint64
	DiskTotal  uint64
}

func GetCPUCores() int {
	return runtime.NumCPU()
}

func GetHostname() string {
	name, err := os.Hostname()
	if err != nil {
		return ""
	}
	return name
}

func GetMemorySizeInMB() uint64 {
	memory, err := memory.VirtualMemory()
	if err != nil {
		return 0
	}
	bytes := memory.Total
	return bytes / (1 << 20)
}

func GetDiskSizeInMB() uint64 {
	usage, err := disk.Usage("/")
	if err != nil {
		return 0
	}
	bytes := usage.Total
	return bytes / (1 << 20)
}

// GetSnapshot returns a point-in-time sample of host resource usage.
// CPU sampling blocks for ~200ms to compute a delta.
func GetSnapshot() Snapshot {
	var s Snapshot

	if pcts, err := cpu.Percent(200*time.Millisecond, false); err == nil && len(pcts) > 0 {
		s.CPUPercent = pcts[0]
	}

	if vm, err := memory.VirtualMemory(); err == nil {
		s.MemUsed = vm.Used
		s.MemTotal = vm.Total
	}

	if du, err := disk.Usage("/"); err == nil {
		s.DiskUsed = du.Used
		s.DiskTotal = du.Total
	}

	return s
}
