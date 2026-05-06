package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/pthsarmah/forge/internal/system"
	ctypes "github.com/pthsarmah/forge/types"
)

// TODO: convert status from string to a validated type later
func SetDeploymentStatus(dep ctypes.Deployment, status string) error {
	baseURL := strings.TrimRight(os.Getenv("CONVEX_SITE_URL"), "/")
	if baseURL == "" {
		return fmt.Errorf("CONVEX_SITE_URL is empty")
	}
	url := baseURL + "/deployments/status"

	body, err := json.Marshal(map[string]any{
		"id":     dep.Id,
		"status": status,
	})
	if err != nil {
		return fmt.Errorf("marshal body: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+dep.NodeToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	res, err := (&http.Client{}).Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer res.Body.Close()

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if res.StatusCode >= 400 {
		return fmt.Errorf("http %d: %s", res.StatusCode, string(raw))
	}

	return nil
}

func GetQueuedDeployments() ([]ctypes.Deployment, error) {
	nodes, err := system.GetAllNodes()
	if err != nil {
		return nil, fmt.Errorf("get all nodes: %w", err)
	}

	baseURL := strings.TrimRight(os.Getenv("CONVEX_SITE_URL"), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("CONVEX_SITE_URL is empty")
	}
	url := baseURL + "/deployments/queued"

	var (
		wg  sync.WaitGroup
		mu  sync.Mutex
		all []ctypes.Deployment
	)

	for _, n := range nodes {
		wg.Add(1)
		go func(info ctypes.NodeInfo) {
			defer wg.Done()
			deps, err := fetchDeploymentsForNode(url, info)
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

func fetchDeploymentsForNode(url string, info ctypes.NodeInfo) ([]ctypes.Deployment, error) {
	body, err := json.Marshal(map[string]any{
		"id": info.NodeId,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal body: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+info.NodeToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	res, err := (&http.Client{}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer res.Body.Close()

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("http %d: %s", res.StatusCode, string(raw))
	}

	var deps []ctypes.Deployment
	if err := json.Unmarshal(raw, &deps); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	//add node token to deployment
	for i := range deps {
		deps[i].NodeToken = info.NodeToken
	}

	return deps, nil
}
