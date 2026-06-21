package api

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/pthsarmah/forge-agent/internal/system"
	ctypes "github.com/pthsarmah/forge-agent/types"
	"github.com/pthsarmah/forge-agent/utils"
)

func SendHeartbeat(ctx context.Context) error {
	logger, _ := utils.GetLoggerInstance()
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	logger.ApiLogger.Println("Heartbeat loop started")

	for {
		select {
		case <-ctx.Done():
			logger.ApiLogger.Printf("Heartbeat loop stopped: %v", ctx.Err())
			return ctx.Err()

		case <-ticker.C:
			nodes, err := system.GetAllNodes()
			if err != nil {
				logger.ApiLogger.Printf("Failed to get nodes: %v", err)
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
						logger.ApiLogger.Printf("Heartbeat failed for user %s: %v", id, err)
					} else {
						logger.ApiLogger.Printf("Heartbeat sent for user %s at %s", id, time.Now().Format(time.RFC3339))
					}

				}(userID, node)
			}

			wg.Wait()
		}
	}
}

func RegisterNode(token string) error {
	logger, _ := utils.GetLoggerInstance()

	cpuCores := system.GetCPUCores()
	memoryMb := system.GetMemorySizeInMB()
	diskMb := system.GetDiskSizeInMB()
	hostname := system.GetHostname()

	logger.ApiLogger.Printf("Registering node hostname=%s cpu=%d mem=%dMB disk=%dMB", hostname, cpuCores, memoryMb, diskMb)

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
		logger.ApiLogger.Printf("Registration failed: %v", err)
		return fmt.Errorf("Registration failed: '%s'\n", err)
	}
	if status != 1 {
		logger.ApiLogger.Printf("Registration failed (status %d): %v", status, errorData)
		return fmt.Errorf("Registration failed: '%s'\n", errorData)
	}

	nodeToken := fmt.Sprintf("%v", successData.Value["nodeToken"])
	userId := fmt.Sprintf("%v", successData.Value["userId"])
	nodeId := fmt.Sprintf("%v", successData.Value["nodeId"])
	cloudflareTunnelToken := fmt.Sprintf("%v", successData.Value["cloudflareTunnelToken"])

	flag, err := system.IsNodeAlreadyConnectedToUser(userId)
	if flag == "connected" {
		logger.ApiLogger.Printf("Node already connected for user %s; deleting duplicate", userId)
		DeleteNode(nodeToken)
		return fmt.Errorf("Node already connected")
	}

	if err != nil {
		logger.ApiLogger.Printf("Node connection check failed: %v", err)
		return fmt.Errorf("%v", err)
	}

	err = system.SetupCloudflared(cloudflareTunnelToken)
	if err != nil {
		logger.SystemLogger.Printf("Setup cloudflared failed: %v", err)
		DeleteNode(nodeToken)
		return fmt.Errorf("Error in saving node: %v", err)
	}

	err = system.SaveNodeInfo(nodeToken, userId, nodeId)

	if err != nil {
		logger.ApiLogger.Printf("Save node info failed: %v", err)
		DeleteNode(nodeToken)
		return fmt.Errorf("Error in saving node: %v", err)
	}

	logger.ApiLogger.Printf("Node registered userId=%s nodeId=%s", userId, nodeId)
	return nil
}

func DeleteNode(nodeToken string) error {
	logger, _ := utils.GetLoggerInstance()
	_, err := CallHttpAction[any]("/nodes/delete-node", nil, true, nodeToken, http.MethodPost)
	if err != nil {
		logger.ApiLogger.Printf("Delete node failed: %v", err)
	} else {
		logger.ApiLogger.Println("Node deleted")
	}
	return err
}
