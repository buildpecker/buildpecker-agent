package api

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/pthsarmah/buildpecker-agent/internal/system"
	ctypes "github.com/pthsarmah/buildpecker-agent/types"
	"github.com/pthsarmah/buildpecker-agent/utils"
)

func GetInfraHealthChecks() ([]ctypes.InfraHealthTarget, error) {
	logger, _ := utils.GetLoggerInstance()
	nodes, err := system.GetAllNodes()
	if err != nil {
		logger.ApiLogger.Printf("Get all nodes failed: %v", err)
		return nil, fmt.Errorf("get all nodes: %w", err)
	}

	path := "/deployments/infra-health"

	var (
		wg  sync.WaitGroup
		mu  sync.Mutex
		all []ctypes.InfraHealthTarget
	)

	for _, n := range nodes {
		wg.Add(1)
		go func(info ctypes.NodeInfo) {
			defer wg.Done()
			data := map[string]any{"id": info.NodeId}
			targets, err := CallHttpAction[[]ctypes.InfraHealthTarget](path, data, true, info.NodeToken, http.MethodPost)
			if err != nil {
				logger.ApiLogger.Printf("Fetch infra health for node %s failed: %v", info.NodeId, err)
				return
			}
			for i := range targets {
				targets[i].NodeToken = info.NodeToken
			}
			mu.Lock()
			all = append(all, targets...)
			mu.Unlock()
		}(n)
	}
	wg.Wait()

	return all, nil
}

func ReportInfraHealth(target ctypes.InfraHealthTarget) error {
	logger, _ := utils.GetLoggerInstance()
	path := fmt.Sprintf("/deployments/health/%s?token=%s", target.DeploymentId, target.HealthToken)
	_, err := CallHttpAction[any](path, nil, false, "", http.MethodGet)
	if err != nil {
		logger.ApiLogger.Printf("Report infra health %s failed: %v", target.DeploymentId, err)
	}
	return err
}
