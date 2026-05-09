package api

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/pthsarmah/forge-agent/internal/system"
	ctypes "github.com/pthsarmah/forge-agent/types"
)

func RegisterNode(token string) error {
	cpuCores := system.GetCPUCores()
	memoryMb := system.GetMemorySizeInMB()
	diskMb := system.GetDiskSizeInMB()
	hostname := system.GetHostname()

	data := ctypes.ConvexRequestBody{
		Path: "nodes/nodejs/actions:registerNode",
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
		return fmt.Errorf("Registration failed: '%s'\n", err)
	}
	if status != 1 {
		return fmt.Errorf("Registration failed: '%s'\n", errorData)
	}

	nodeToken := fmt.Sprintf("%v", successData.Value["nodeToken"])
	userId := fmt.Sprintf("%v", successData.Value["userId"])
	nodeId := fmt.Sprintf("%v", successData.Value["nodeId"])

	flag, err := system.IsNodeAlreadyConnectedToUser(userId)
	if flag == "connected" {
		fmt.Print("Node already exists for this user. Deleting new entry...")
		DeleteNode(nodeId, nodeToken)
		return fmt.Errorf("Node already connected")
	}

	if err != nil {
		return fmt.Errorf("%v", err)
	}

	err = system.SaveNodeInfo(nodeToken, userId, nodeId)

	if err != nil {
		DeleteNode(nodeId, nodeToken)
		return fmt.Errorf("Error in saving node: %v", err)
	}

	fmt.Println("Node successfully registered!")
	return nil
}

func DeleteNode(nodeId string, nodeToken string) error {
	client := http.Client{}
	baseUrl := strings.TrimRight(os.Getenv("CONVEX_SITE_URL"), "/")
	url := baseUrl + "/nodes/delete-node"

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("Error creating request: %s", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", nodeToken))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Error in generating response '%s'", err)
	}
	defer res.Body.Close()

	_, err = io.ReadAll(res.Body)
	if err != nil {
	}

	if res.StatusCode >= 400 {
		return fmt.Errorf("Error in deleting node")
	}

	return nil
}
