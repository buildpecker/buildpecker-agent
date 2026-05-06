package deploy

import (
	"fmt"
	"os"

	"github.com/pthsarmah/forge/internal/api"
	"github.com/pthsarmah/forge/internal/docker"
	ctypes "github.com/pthsarmah/forge/types"
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
			err := docker.Deploy(dep)
			if err != nil {
				return
			}
			//set deployment status to processing
			api.SetDeploymentStatus(dep, "processing")
		default:
			fmt.Fprintf(os.Stderr, "Invalid deployment provided for start_deploy")
			return
		}
	}
}
