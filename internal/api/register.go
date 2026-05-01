package api

import (
	"fmt"
	"os"

	"github.com/pthsarmah/forge/internal/system"
)

func RegisterNode(token string) {
	cpuCores := system.GetCPUCores()
	memoryMb := system.GetMemorySizeInMB()
	diskMb := system.GetDiskSizeInMB()
	hostname := system.GetHostname()

	data := map[string]any{
		"path": "nodes/actions:registerNode",
		"args": map[string]any{
			"token":    token,
			"cpuCores": cpuCores,
			"memoryMb": memoryMb,
			"diskMb":   diskMb,
			"hostname": hostname,
		},
		"format": "json",
	}

	status, _, errorData := Post("api/action", "application/json", data)
	if status != 1 {
		fmt.Fprintf(os.Stderr, "Registration failed: '%s'\n", errorData)
		return
	}

	fmt.Println("Node successfully registered!")
}
