package api

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/pthsarmah/forge-agent/internal/system"
	ctypes "github.com/pthsarmah/forge-agent/types"
	"github.com/pthsarmah/forge-agent/utils"
)

// TODO: convert 'status' from string to a validated type later
func SetDeploymentStatus(dep ctypes.Deployment, status string, localPort int) error {
	logger, _ := utils.GetLoggerInstance()
	var data = map[string]any{
		"id":     dep.Id,
		"status": status,
	}
	if localPort > 0 {
		data["localPort"] = localPort
	}
	_, err := CallHttpAction[any]("/deployments/status", data, true,
		dep.NodeToken, http.MethodPatch)

	if err != nil {
		logger.ApiLogger.Printf("Set deployment %s status=%s failed: %v", dep.Id, status, err)
	} else {
		logger.ApiLogger.Printf("Deployment %s status=%s", dep.Id, status)
	}
	return err
}

func GetQueuedDeployments() ([]ctypes.Deployment, error) {
	logger, _ := utils.GetLoggerInstance()
	nodes, err := system.GetAllNodes()
	if err != nil {
		logger.ApiLogger.Printf("Get all nodes failed: %v", err)
		return nil, fmt.Errorf("get all nodes: %w", err)
	}

	path := "/deployments/queued"

	var (
		wg  sync.WaitGroup
		mu  sync.Mutex
		all []ctypes.Deployment
	)

	for _, n := range nodes {
		wg.Add(1)
		go func(info ctypes.NodeInfo) {
			defer wg.Done()
			deps, err := fetchDeploymentsForNode(path, info)
			if err != nil {
				logger.ApiLogger.Printf("Fetch queued for node %s failed: %v", info.NodeId, err)
				return
			}
			mu.Lock()
			all = append(all, deps...)
			mu.Unlock()
		}(n)
	}
	wg.Wait()

	if len(all) > 0 {
		logger.ApiLogger.Printf("Fetched %d queued deployments", len(all))
	}
	return all, nil
}

func fetchDeploymentsForNode(path string, info ctypes.NodeInfo) ([]ctypes.Deployment, error) {

	var data = map[string]any{
		"id": info.NodeId,
	}

	deps, err := CallHttpAction[[]ctypes.Deployment](path, data, true, info.NodeToken, http.MethodPost)

	if err != nil {
		return nil, err
	}

	//add node token to deployment
	for i := range deps {
		deps[i].NodeToken = info.NodeToken
	}

	return deps, nil
}

func GetPendingDeletes() ([]ctypes.Deployment, error) {
	logger, _ := utils.GetLoggerInstance()
	nodes, err := system.GetAllNodes()
	if err != nil {
		logger.ApiLogger.Printf("Get all nodes failed: %v", err)
		return nil, fmt.Errorf("get all nodes: %w", err)
	}

	path := "/deployments/pending-deletes"

	var (
		wg  sync.WaitGroup
		mu  sync.Mutex
		all []ctypes.Deployment
	)

	for _, n := range nodes {
		wg.Add(1)
		go func(info ctypes.NodeInfo) {
			defer wg.Done()
			deps, err := fetchDeploymentsForNode(path, info)
			if err != nil {
				logger.ApiLogger.Printf("Fetch pending deletes for node %s failed: %v", info.NodeId, err)
				return
			}
			mu.Lock()
			all = append(all, deps...)
			mu.Unlock()
		}(n)
	}
	wg.Wait()

	if len(all) > 0 {
		logger.ApiLogger.Printf("Fetched %d pending deletes", len(all))
	}
	return all, nil
}

func FinalizeDelete(dep ctypes.Deployment) error {
	logger, _ := utils.GetLoggerInstance()
	data := map[string]any{
		"id": dep.Id,
	}
	_, err := CallHttpAction[any]("/deployments/finalize-delete", data, true, dep.NodeToken, http.MethodPost)
	if err != nil {
		logger.ApiLogger.Printf("Finalize delete %s failed: %v", dep.Id, err)
	} else {
		logger.ApiLogger.Printf("Finalized delete %s", dep.Id)
	}
	return err
}
