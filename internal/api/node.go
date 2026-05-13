package api

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/pthsarmah/forge-agent/internal/system"
	ctypes "github.com/pthsarmah/forge-agent/types"
)

func SendHeartbeat(ctx context.Context) error {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-ticker.C:
			nodes, err := system.GetAllNodes()
			if err != nil {
				fmt.Printf("failed to get nodes: %v\n", err)
				continue
			}

			var wg sync.WaitGroup

			for userID, node := range nodes {
				wg.Add(1)

				go func(id string, n ctypes.NodeInfo) {
					defer wg.Done()

					_, err := CallHttpAction[any](
						"/nodes/heartbeat",
						nil,
						true,
						n.NodeToken,
						http.MethodPost,
					)

					if err != nil {
						fmt.Printf(
							"heartbeat failed for %s: %v\n",
							id,
							err,
						)
					} else {
						fmt.Printf("heartbeat sent for user %s\n at time: %s", id, time.Now())
					}

				}(userID, node)
			}

			wg.Wait()
		}
	}
}

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
		DeleteNode(nodeToken)
		return fmt.Errorf("Node already connected")
	}

	if err != nil {
		return fmt.Errorf("%v", err)
	}

	err = system.SaveNodeInfo(nodeToken, userId, nodeId)

	if err != nil {
		DeleteNode(nodeToken)
		return fmt.Errorf("Error in saving node: %v", err)
	}

	fmt.Println("Node successfully registered!")
	return nil
}

func DeleteNode(nodeToken string) error {
	_, err := CallHttpAction[any]("/nodes/delete-node", nil, true, nodeToken, http.MethodPost)
	return err
}
