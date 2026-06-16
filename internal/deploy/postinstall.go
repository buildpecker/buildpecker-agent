package deploy

import (
	"os/exec"

	"github.com/pthsarmah/forge-agent/internal/api"
	ctypes "github.com/pthsarmah/forge-agent/types"
	"github.com/pthsarmah/forge-agent/utils"
)

func handlePostInstall(run ctypes.PostInstallRun) {
	logger, _ := utils.GetLoggerInstance()

	var cmd *exec.Cmd
	if run.Type == "project" {
		container := deriveImageName(run.RepoUrl)
		logger.DeployLogger.Printf("Postinstall run=%s name=%q container=%s", run.Id, run.Name, container)
		cmd = exec.Command(
			"docker", "exec", "-i", container,
			"sh", "-c", run.Command,
		)
	} else {
		project := composeProjectName(run.ContainerName, run.DeploymentId)
		logger.DeployLogger.Printf("Postinstall run=%s name=%q project=%s service=%s", run.Id, run.Name, project, run.Service)
		cmd = exec.Command(
			"docker", "compose", "-p", project,
			"exec", "-T", run.Service,
			"sh", "-c", run.Command,
		)
	}

	out, err := cmd.CombinedOutput()
	exitCode := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		} else {
			exitCode = 1
			out = append(out, []byte("\n"+err.Error())...)
		}
	}

	if err := api.SetPostInstallResult(run, string(out), exitCode); err != nil {
		logger.DeployLogger.Printf("Postinstall result report failed run=%s: %v", run.Id, err)
	}
	logger.DeployLogger.Printf("Postinstall run=%s name=%q finished exit=%d", run.Id, run.Name, exitCode)
}
