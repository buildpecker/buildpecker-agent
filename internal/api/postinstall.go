package api

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/pthsarmah/forge-agent/internal/system"
	ctypes "github.com/pthsarmah/forge-agent/types"
	"github.com/pthsarmah/forge-agent/utils"
)

func GetQueuedPostInstalls() ([]ctypes.PostInstallRun, error) {
	logger, _ := utils.GetLoggerInstance()
	nodes, err := system.GetAllNodes()
	if err != nil {
		logger.ApiLogger.Printf("Get all nodes failed: %v", err)
		return nil, fmt.Errorf("get all nodes: %w", err)
	}

	path := "/postinstall/queued"

	var (
		wg  sync.WaitGroup
		mu  sync.Mutex
		all []ctypes.PostInstallRun
	)

	for _, n := range nodes {
		wg.Add(1)
		go func(info ctypes.NodeInfo) {
			defer wg.Done()
			data := map[string]any{"id": info.NodeId}
			runs, err := CallHttpAction[[]ctypes.PostInstallRun](path, data, true, info.NodeToken, http.MethodPost)
			if err != nil {
				logger.ApiLogger.Printf("Fetch postinstall for node %s failed: %v", info.NodeId, err)
				return
			}
			for i := range runs {
				runs[i].NodeToken = info.NodeToken
			}
			mu.Lock()
			all = append(all, runs...)
			mu.Unlock()
		}(n)
	}
	wg.Wait()

	if len(all) > 0 {
		logger.ApiLogger.Printf("Fetched %d queued postinstall runs", len(all))
	}
	return all, nil
}

func SetPostInstallResult(run ctypes.PostInstallRun, output string, exitCode int) error {
	logger, _ := utils.GetLoggerInstance()
	data := map[string]any{
		"id":       run.Id,
		"output":   output,
		"exitCode": exitCode,
	}
	_, err := CallHttpAction[any]("/postinstall/result", data, true, run.NodeToken, http.MethodPost)
	if err != nil {
		logger.ApiLogger.Printf("Set postinstall result %s failed: %v", run.Id, err)
	} else {
		logger.ApiLogger.Printf("Postinstall %s exit=%d", run.Id, exitCode)
	}
	return err
}
