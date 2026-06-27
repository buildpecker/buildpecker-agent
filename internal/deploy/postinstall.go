package deploy

import (
	"context"
	"os/exec"
	"time"

	"github.com/pthsarmah/buildpecker-agent/internal/api"
	ctypes "github.com/pthsarmah/buildpecker-agent/types"
	"github.com/pthsarmah/buildpecker-agent/utils"
)

const postInstallTimeout = 10 * time.Minute

func handlePostInstall(run ctypes.PostInstallRun) {
	logger, _ := utils.GetLoggerInstance()

	ctx, cancel := context.WithTimeout(context.Background(), postInstallTimeout)
	defer cancel()

	var cmd *exec.Cmd
	if run.Type == "project" {
		container := deriveImageName(run.RepoUrl)
		logger.DeployLogger.Printf("Postinstall run=%s name=%q container=%s", run.Id, run.Name, container)
		cmd = exec.CommandContext(
			ctx,
			"docker", "exec", "-i", container,
			"sh", "-c", run.Command,
		)
	} else {
		project := composeProjectName(run.ContainerName, run.DeploymentId)
		logger.DeployLogger.Printf("Postinstall run=%s name=%q project=%s service=%s", run.Id, run.Name, project, run.Service)
		cmd = exec.CommandContext(
			ctx,
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
		if ctx.Err() == context.DeadlineExceeded {
			out = append(out, []byte("\ncommand timed out after "+postInstallTimeout.String())...)
		}
	}

	if err := api.SetPostInstallResult(run, string(out), exitCode); err != nil {
		logger.DeployLogger.Printf("Postinstall result report failed run=%s: %v", run.Id, err)
	}
	logger.DeployLogger.Printf("Postinstall run=%s name=%q finished exit=%d", run.Id, run.Name, exitCode)
}
