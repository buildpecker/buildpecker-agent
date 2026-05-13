package api

import (
	ctypes "github.com/pthsarmah/forge-agent/types"
	"net/http"
)

func GetEnvironmentSecrets(dep ctypes.Deployment) ([]ctypes.EnvVar, error) {

	var data = map[string]any{
		"projectId": dep.Project.Id,
	}

	envMap, err := CallHttpAction[[]ctypes.EnvVar]("/environments/secrets", data, true, dep.NodeToken, http.MethodPost)

	if err != nil {
		return nil, err
	}

	return envMap, nil
}
