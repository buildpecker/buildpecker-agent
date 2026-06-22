package api

import (
	ctypes "github.com/pthsarmah/buildpecker-agent/types"
	"github.com/pthsarmah/buildpecker-agent/utils"
	"net/http"
)

func SetProjectFramework(dep ctypes.Deployment, framework ctypes.FrameworkInfo) error {
	logger, _ := utils.GetLoggerInstance()

	fwString := framework.DisplayName

	var data = map[string]any{
		"id":        dep.Project.Id,
		"framework": fwString,
	}

	_, err := CallHttpAction[any]("/projects/framework", data, true, dep.NodeToken, http.MethodPatch)

	if err != nil {
		logger.ApiLogger.Printf("Set project %s framework=%s failed: %v", dep.Project.Id, fwString, err)
	} else {
		logger.ApiLogger.Printf("Project %s framework=%s", dep.Project.Id, fwString)
	}
	return err
}
