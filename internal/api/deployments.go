package api

import (
	"fmt"
	"net/http"
	"os"
	"sync"

	"github.com/pthsarmah/forge-agent/internal/system"
	ctypes "github.com/pthsarmah/forge-agent/types"
)

// TODO: convert 'status' from string to a validated type later
func SetDeploymentStatus(dep ctypes.Deployment, status string) error {
	var data = map[string]any{
		"id":     dep.Id,
		"status": status,
	}
	_, err := CallHttpAction[any]("/deployments/status", data, true,
		dep.NodeToken, http.MethodPatch)

	return err
}

func GetQueuedDeployments() ([]ctypes.Deployment, error) {
	nodes, err := system.GetAllNodes()
	if err != nil {
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
				fmt.Fprintf(os.Stderr, "fetch queued for node %s: %v\n", info.NodeId, err)
				return
			}
			mu.Lock()
			all = append(all, deps...)
			mu.Unlock()
		}(n)
	}
	wg.Wait()

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
