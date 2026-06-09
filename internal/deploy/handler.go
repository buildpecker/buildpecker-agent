package deploy

import (
	"log"
	"os/exec"
	"path"
	"strings"

	"github.com/pthsarmah/forge-agent/internal/api"
	//	"github.com/pthsarmah/forge-agent/internal/docker"
	"github.com/pthsarmah/forge-agent/internal/git"
	ctypes "github.com/pthsarmah/forge-agent/types"
	"github.com/pthsarmah/forge-agent/utils"
)

func deriveImageName(repoUrl string) string {
	return strings.ToLower(strings.TrimSuffix(path.Base(repoUrl), ".git"))
}

func Handler(event string, args ...any) {
	logger, _ := utils.GetLoggerInstance()

	switch event {
	case "postinstall":
		if len(args) != 1 {
			logger.DeployLogger.Println("Invalid no. of args for postinstall")
			return
		}
		run, ok := args[0].(ctypes.PostInstallRun)
		if !ok {
			logger.DeployLogger.Println("Invalid run provided for postinstall")
			return
		}
		handlePostInstall(run)
		return

	case "infra_health":
		if len(args) != 1 {
			logger.DeployLogger.Println("Invalid no. of args for infra_health")
			return
		}
		target, ok := args[0].(ctypes.InfraHealthTarget)
		if !ok {
			logger.DeployLogger.Println("Invalid target provided for infra_health")
			return
		}
		handleInfraHealth(target)
		return

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

			if dep.Type == "infra" {
				handleInfraDeploy(dep, depLog, logger)
				return
			}

			if dep.ProjectId == "" || dep.Project.RepoUrl == "" {
				logger.DeployLogger.Printf("Malformed project deployment dep=%s type=%q projectId=%q repo=%q; failing",
					dep.Id, dep.Type, dep.ProjectId, dep.Project.RepoUrl)
				if depLog != nil {
					depLog.Println("Malformed deployment: missing project/repo; marking failed")
				}
				setStatus(dep, depLog, logger, "failed", 0)
				return
			}

			logger.DeployLogger.Printf("Handling deployment %s repo=%s", dep.Id, dep.Project.RepoUrl)
			if depLog != nil {
				depLog.Printf("Handling deployment repo=%s", dep.Project.RepoUrl)
			}

			//set deployment status to processing
			err := api.SetDeploymentStatus(dep, "processing", 0)
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
				setStatus(dep, depLog, logger, "failed", 0)
				return
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
				setStatus(dep, depLog, logger, "failed", 0)
				return
			}
			logger.DeployLogger.Printf("Detected framework dep=%s framework=%s", dep.Id, framework.DisplayName)
			if depLog != nil {
				depLog.Printf("Detected framework: %s", framework.DisplayName)
			}

			//set repo framework
			err = api.SetProjectFramework(dep, framework)
			if err != nil {
				logger.DeployLogger.Printf("Set project framework failed dep=%s: %v", dep.Id, err)
				if depLog != nil {
					depLog.Printf("Set project framework failed: %v", err)
				}
				setStatus(dep, depLog, logger, "failed", 0)
				return
			}

			envs, err := api.GetEnvironmentSecrets(dep)
			if err != nil {
				logger.DeployLogger.Printf("Get env secrets failed dep=%s: %v", dep.Id, err)
				if depLog != nil {
					depLog.Printf("Get env secrets failed: %v", err)
				}
				setStatus(dep, depLog, logger, "failed", 0)
				return
			}
			if depLog != nil {
				depLog.Printf("Fetched %d env secrets", len(envs))
			}

			//deploy
			hostPort, err := NixpackDeploy(dep, envs, path, framework)
			if err != nil {
				logger.DeployLogger.Printf("Nixpack deploy failed dep=%s: %v", dep.Id, err)
				if depLog != nil {
					depLog.Printf("Nixpack deploy failed: %v", err)
				}
				setStatus(dep, depLog, logger, "failed", 0)
				return
			}

			//set deployment status to completed
			setStatus(dep, depLog, logger, "completed", hostPort)

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

	case "start_delete":
		if len(args) != 1 {
			logger.DeployLogger.Println("Invalid no. of args for start_delete")
			return
		}

		dep, ok := args[0].(ctypes.Deployment)
		if !ok {
			logger.DeployLogger.Println("Invalid deployment provided for start_delete")
			return
		}

		if dep.Type == "infra" {
			if err := InfraDelete(dep); err != nil {
				logger.DeployLogger.Printf("Infra delete dep=%s failed: %v", dep.Id, err)
			}
			if err := api.FinalizeDelete(dep); err != nil {
				logger.DeployLogger.Printf("Finalize delete dep=%s failed: %v", dep.Id, err)
			}
			return
		}

		imageName := deriveImageName(dep.Project.RepoUrl)
		logger.DeployLogger.Printf("Stopping container for delete dep=%s image=%s", dep.Id, imageName)

		rmCmd := exec.Command("docker", "rm", "-f", imageName)
		if out, err := rmCmd.CombinedOutput(); err != nil {
			logger.DeployLogger.Printf("docker rm dep=%s image=%s failed: %v output=%s",
				dep.Id, imageName, err, strings.TrimSpace(string(out)))
		} else {
			logger.DeployLogger.Printf("docker rm dep=%s image=%s ok", dep.Id, imageName)
		}

		if err := api.FinalizeDelete(dep); err != nil {
			logger.DeployLogger.Printf("Finalize delete dep=%s failed: %v", dep.Id, err)
		}
	}
}

func handleInfraDeploy(dep ctypes.Deployment, depLog *log.Logger, logger *utils.Logger) {
	logger.DeployLogger.Printf("Handling infra deployment %s template=%s", dep.Id, dep.Infra.Template.Identifier)
	if depLog != nil {
		depLog.Printf("Handling infra deployment template=%s", dep.Infra.Template.Identifier)
	}

	if err := api.SetDeploymentStatus(dep, "processing", 0); err != nil {
		logger.DeployLogger.Printf("Set status processing failed dep=%s: %v", dep.Id, err)
		if depLog != nil {
			depLog.Printf("Set status processing failed: %v", err)
		}
	}

	envs, err := api.GetEnvironmentSecrets(dep)
	if err != nil {
		logger.DeployLogger.Printf("Get infra env secrets failed dep=%s: %v", dep.Id, err)
		if depLog != nil {
			depLog.Printf("Get env secrets failed: %v", err)
		}
		setStatus(dep, depLog, logger, "failed", 0)
		return
	}
	if depLog != nil {
		depLog.Printf("Fetched %d env secrets", len(envs))
	}

	portMap, err := InfraDeploy(dep, envs)
	if err != nil {
		logger.DeployLogger.Printf("Infra deploy failed dep=%s: %v", dep.Id, err)
		if depLog != nil {
			depLog.Printf("Infra deploy failed: %v", err)
		}
		setStatus(dep, depLog, logger, "failed", 0)
		return
	}

	if err := api.SetInfraDeploymentStatus(dep, "completed", portMap); err != nil {
		logger.DeployLogger.Printf("Set infra status completed failed dep=%s: %v", dep.Id, err)
		if depLog != nil {
			depLog.Printf("Set status completed failed: %v", err)
		}
	}
	logger.DeployLogger.Printf("Infra deployment %s done", dep.Id)
	if depLog != nil {
		depLog.Println("Infra deployment done")
	}
}

func setStatus(dep ctypes.Deployment, depLog *log.Logger, logger *utils.Logger, status string, hostPort int) {
	err := api.SetDeploymentStatus(dep, status, hostPort)
	if err != nil {
		logger.DeployLogger.Printf("Set status completed failed dep=%s: %v", dep.Id, err)
		if depLog != nil {
			depLog.Printf("Set status completed failed: %v", err)
		}
	}
}
