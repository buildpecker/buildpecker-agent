package api

import (
	"fmt"
	"os"

	"github.com/pthsarmah/forge/internal/system"
	ctypes "github.com/pthsarmah/forge/types"
)

func RegisterNode(token string) {
	cpuCores := system.GetCPUCores()
	memoryMb := system.GetMemorySizeInMB()
	diskMb := system.GetDiskSizeInMB()
	hostname := system.GetHostname()

	data := ctypes.ConvexRequestBody{
		Path: "nodes/actions:registerNode",
		Args: map[string]any{
			"token":    token,
			"cpuCores": cpuCores,
			"memoryMb": memoryMb,
			"diskMb":   diskMb,
			"hostname": hostname,
		},
		Format: "json",
	}

	status, successData, errorData, err := Post("api/action", "application/json", data, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Registration failed: '%s'\n", err)
		return
	}
	if status != 1 {
		fmt.Fprintf(os.Stderr, "Registration failed: '%s'\n", errorData)
		return
	}

	nodeToken := fmt.Sprintf("%v", successData.Value["nodeToken"])
	userId := fmt.Sprintf("%v", successData.Value["userId"])
	nodeId := fmt.Sprintf("%v", successData.Value["nodeId"])

	err = system.SaveNodeInfo(nodeToken, userId, nodeId)

	if err != nil {
		DeleteNode(nodeId)
		return
	}

	fmt.Println("Node successfully registered!")
}

func DeleteNode(nodeId string)
