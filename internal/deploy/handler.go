package deploy

import (
	"fmt"
	"os"

	"github.com/pthsarmah/forge-agent/internal/api"
	"github.com/pthsarmah/forge-agent/internal/docker"
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
			//clone repo if not already cloned
			path, err := git.CloneRepo(dep.Project.RepoUrl)
			if err != nil && path == "" {
				fmt.Fprintf(os.Stderr, "%v", err)
			}

			//detect repo framework
			framework, err := DetectFramework(dep, path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v", err)
			}
			fmt.Printf("This project is based on %s framework\n", framework)

			//set repo framework
			err = api.SetProjectFramework(dep, framework)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v", err)
			}

			envs, err := api.GetEnvironmentSecrets(dep)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v", err)
			}
			fmt.Printf("Got environment secrets!\n")

			//set deployment status to processing
			err = api.SetDeploymentStatus(dep, "processing")
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v", err)
			}

			//deploy
			err = docker.Deploy(dep, envs, path, framework)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v", err)
			}

			//set deployment status to completed
			err = api.SetDeploymentStatus(dep, "completed")
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v", err)
			}

		default:
			fmt.Fprintf(os.Stderr, "Invalid deployment provided for start_deploy")
			return
		}
	}
}
