package api

import (
	"net/http"

	ctypes "github.com/pthsarmah/forge-agent/types"
	"github.com/pthsarmah/forge-agent/utils"
)

func GetEnvironmentSecrets(dep ctypes.Deployment) ([]ctypes.EnvVar, error) {
	logger, _ := utils.GetLoggerInstance()

	var data = map[string]any{
		"projectId": dep.Project.Id,
	}

	envMap, err := CallHttpAction[[]ctypes.EnvVar]("/environments/secrets", data, true, dep.NodeToken, http.MethodPost)

	if err != nil {
		logger.ApiLogger.Printf("Fetch env secrets for project %s failed: %v", dep.Project.Id, err)
		return nil, err
	}

	logger.ApiLogger.Printf("Fetched %d env secrets for project %s", len(envMap), dep.Project.Id)
	return envMap, nil
}
