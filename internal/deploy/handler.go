package deploy

import (
	"github.com/pthsarmah/forge-agent/internal/api"
	//	"github.com/pthsarmah/forge-agent/internal/docker"
	"github.com/pthsarmah/forge-agent/internal/git"
	ctypes "github.com/pthsarmah/forge-agent/types"
	"github.com/pthsarmah/forge-agent/utils"
)

func Handler(event string, args ...any) {
	logger, _ := utils.GetLoggerInstance()

	switch event {
	case "start_deploy":
		if len(args) == 0 {
			logger.DeployLogger.Println("No deployment provided for start_deploy")
			return
		}

		if len(args) > 1 {
			logger.DeployLogger.Println("Invalid no. of deployments provided for start_deploy")
			return
		}

		switch args[0].(type) {
		case ctypes.Deployment:
			dep := args[0].(ctypes.Deployment)
			depLog, _ := logger.GetDeploymentLogger(dep.Id)

			logger.DeployLogger.Printf("Handling deployment %s repo=%s", dep.Id, dep.Project.RepoUrl)
			if depLog != nil {
				depLog.Printf("Handling deployment repo=%s", dep.Project.RepoUrl)
			}

			//set deployment status to processing
			err := api.SetDeploymentStatus(dep, "processing")
			if err != nil {
				logger.DeployLogger.Printf("Set status processing failed dep=%s: %v", dep.Id, err)
				if depLog != nil {
					depLog.Printf("Set status processing failed: %v", err)
				}
			}

			var status = "completed"

			//clone repo if not already cloned
			path, err := git.CloneRepo(dep.Project.RepoUrl, dep.Id)
			if err != nil && path == "" {
				logger.DeployLogger.Printf("Clone repo failed dep=%s: %v", dep.Id, err)
				if depLog != nil {
					depLog.Printf("Clone repo failed: %v", err)
				}
				status = "failed"
			}
			if depLog != nil {
				depLog.Printf("Repo cloned at %s", path)
			}

			//detect repo framework
			framework, err := DetectFramework(dep, path)
			if err != nil {
				logger.DeployLogger.Printf("Detect framework failed dep=%s: %v", dep.Id, err)
				if depLog != nil {
					depLog.Printf("Detect framework failed: %v", err)
				}
				status = "failed"
			}
			logger.DeployLogger.Printf("Detected framework dep=%s framework=%s", dep.Id, framework)
			if depLog != nil {
				depLog.Printf("Detected framework: %s", framework)
			}

			//set repo framework
			err = api.SetProjectFramework(dep, framework)
			if err != nil {
				logger.DeployLogger.Printf("Set project framework failed dep=%s: %v", dep.Id, err)
				if depLog != nil {
					depLog.Printf("Set project framework failed: %v", err)
				}
				status = "failed"
			}

			envs, err := api.GetEnvironmentSecrets(dep)
			if err != nil {
				logger.DeployLogger.Printf("Get env secrets failed dep=%s: %v", dep.Id, err)
				if depLog != nil {
					depLog.Printf("Get env secrets failed: %v", err)
				}
				status = "failed"
			}
			if depLog != nil {
				depLog.Printf("Fetched %d env secrets", len(envs))
			}

			//deploy
			err = NixpackDeploy(dep, envs, path, framework)
			if err != nil {
				logger.DeployLogger.Printf("Nixpack deploy failed dep=%s: %v", dep.Id, err)
				if depLog != nil {
					depLog.Printf("Nixpack deploy failed: %v", err)
				}
				status = "failed"
			}

			//set deployment status to completed
			err = api.SetDeploymentStatus(dep, status)
			if err != nil {
				logger.DeployLogger.Printf("Set status completed failed dep=%s: %v", dep.Id, err)
				if depLog != nil {
					depLog.Printf("Set status completed failed: %v", err)
				}
			}

			if status == "completed" {
				logger.DeployLogger.Printf("Deployment %s done", dep.Id)
				if depLog != nil {
					depLog.Println("Deployment done")
				}
			}

		default:
			logger.DeployLogger.Println("Invalid deployment provided for start_deploy")
			return
		}
	}
}
