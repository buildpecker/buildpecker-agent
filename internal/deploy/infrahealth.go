package deploy

import (
	"os/exec"
	"strings"

	"github.com/pthsarmah/buildpecker-agent/internal/api"
	ctypes "github.com/pthsarmah/buildpecker-agent/types"
	"github.com/pthsarmah/buildpecker-agent/utils"
)

func handleInfraHealth(target ctypes.InfraHealthTarget) {
	logger, _ := utils.GetLoggerInstance()

	project := composeProjectName(target.ContainerName, target.DeploymentId)

	var cmd *exec.Cmd
	if strings.TrimSpace(target.Service) == "" {
		cmd = exec.Command("sh", "-c", target.Command)
	} else {
		cmd = exec.Command(
			"docker", "compose", "-p", project,
			"exec", "-T", target.Service,
			"sh", "-c", target.Command,
		)
	}

	if err := cmd.Run(); err != nil {
		logger.DeployLogger.Printf("Infra health dep=%s project=%s unhealthy: %v", target.DeploymentId, project, err)
		return
	}

	if err := api.ReportInfraHealth(target); err != nil {
		logger.DeployLogger.Printf("Infra health report dep=%s failed: %v", target.DeploymentId, err)
	}
}
