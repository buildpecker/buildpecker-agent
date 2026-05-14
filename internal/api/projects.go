package api

import (
	"net/http"

	ctypes "github.com/pthsarmah/forge-agent/types"
	"github.com/pthsarmah/forge-agent/utils"
)

func SetProjectFramework(dep ctypes.Deployment, framework string) error {
	logger, _ := utils.GetLoggerInstance()

	var data = map[string]any{
		"id":        dep.Project.Id,
		"framework": framework,
	}

	_, err := CallHttpAction[any]("/projects/framework", data, true, dep.NodeToken, http.MethodPatch)

	if err != nil {
		logger.ApiLogger.Printf("Set project %s framework=%s failed: %v", dep.Project.Id, framework, err)
	} else {
		logger.ApiLogger.Printf("Project %s framework=%s", dep.Project.Id, framework)
	}
	return err
}
