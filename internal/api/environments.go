package api

import (
	"net/http"

	ctypes "github.com/pthsarmah/buildpecker-agent/types"
	"github.com/pthsarmah/buildpecker-agent/utils"
)

func GetEnvironmentSecrets(dep ctypes.Deployment) ([]ctypes.EnvVar, error) {
	logger, _ := utils.GetLoggerInstance()

	var data map[string]any
	var scope string
	if dep.Type == "infra" {
		data = map[string]any{"infraId": dep.Infra.Id}
		scope = "infra " + dep.Infra.Id
	} else {
		data = map[string]any{"projectId": dep.Project.Id}
		scope = "project " + dep.Project.Id
	}

	envMap, err := CallHttpAction[[]ctypes.EnvVar]("/environments/secrets", data, true, dep.NodeToken, http.MethodPost)

	if err != nil {
		logger.ApiLogger.Printf("Fetch env secrets for %s failed: %v", scope, err)
		return nil, err
	}

	logger.ApiLogger.Printf("Fetched %d env secrets for %s", len(envMap), scope)
	return envMap, nil
}
