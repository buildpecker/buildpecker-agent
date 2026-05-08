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

func RegisterNode(token string) {
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

	flag, err := system.IsNodeAlreadyConnectedToUser(userId)
	if err != nil || flag == false {
		fmt.Print("Node already exists for this user. Deleting new entry...")
		DeleteNode(nodeId, nodeToken)
		return
	}

	err = system.SaveNodeInfo(nodeToken, userId, nodeId)

	if err != nil {
		DeleteNode(nodeId, nodeToken)
		return
	}

	fmt.Println("Node successfully registered!")
}

func DeleteNode(nodeId string, nodeToken string) {
	client := http.Client{}
	baseUrl := strings.TrimRight(os.Getenv("CONVEX_SITE_URL"), "/")
	url := baseUrl + "/nodes/delete-node"

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating request: %s", err)
		return
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", nodeToken))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	res, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error in generating response '%s'", err)
		return
	}
	defer res.Body.Close()

	_, err = io.ReadAll(res.Body)
	if err != nil {
		return
	}

	if res.StatusCode >= 400 {
		fmt.Fprintf(os.Stderr, "Error in deleting node")
	}
}
