package system

import (
	disk "github.com/shirou/gopsutil/v4/disk"
	memory "github.com/shirou/gopsutil/v4/mem"
	"os"
	"runtime"
)

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
