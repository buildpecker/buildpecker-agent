package deploy

import (
	"fmt"
	"os"
	"strings"

	"github.com/pthsarmah/forge-agent/internal/api"
	//	"github.com/pthsarmah/forge-agent/internal/docker"
	"github.com/pthsarmah/forge-agent/internal/git"
	ctypes "github.com/pthsarmah/forge-agent/types"
)

func Handler(event string, args ...any) {
	switch event {
	case "start_deploy":
		if len(args) == 0 {
			fmt.Fprintf(os.Stderr, "No deployment provided for start_deploy")
			return
		}

		if len(args) > 1 {
			fmt.Fprintf(os.Stderr, "Invalid no. of deployments provided for start_deploy")
			return
		}

		switch args[0].(type) {
		case ctypes.Deployment:
			dep := args[0].(ctypes.Deployment)
			//			err := docker.Deploy(dep)
			//			if err != nil {
			//				return
			//			}
			//set deployment status to processing
			api.SetDeploymentStatus(dep, "processing")

			//clone repo if not already cloned
			pathEles := strings.Split(dep.Project.RepoUrl, "/")
			projectEles := strings.Split(pathEles[len(pathEles)-1], ".git")
			path := projectEles[0]
			git.CloneRepo(dep.Project.RepoUrl, path)

		default:
			fmt.Fprintf(os.Stderr, "Invalid deployment provided for start_deploy")
			return
		}
	}
}
