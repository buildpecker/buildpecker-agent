package api

import (
	ctypes "github.com/pthsarmah/forge-agent/types"
	"net/http"
)

func SetProjectFramework(dep ctypes.Deployment, framework string) error {

	var data = map[string]any{
		"id":        dep.Project.Id,
		"framework": framework,
	}

	_, err := CallHttpAction[any]("/projects/framework", data, true, dep.NodeToken, http.MethodPatch)

	return err
}
