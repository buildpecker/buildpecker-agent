package deploy

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"os/user"

	"github.com/pthsarmah/buildpecker-agent/internal/api"
	ctypes "github.com/pthsarmah/buildpecker-agent/types"
	"github.com/pthsarmah/buildpecker-agent/utils"
)

const healthCommandTimeout = 30 * time.Second

func unprivilegedCredential() (*syscall.Credential, error) {
	u, err := user.Lookup("nobody")
	if err != nil {
		return nil, err
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return nil, err
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return nil, err
	}
	return &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}, nil
}

func handleInfraHealth(target ctypes.InfraHealthTarget) {
	logger, _ := utils.GetLoggerInstance()
	depLog, _ := logger.GetDeploymentLogger(target.DeploymentId)

	project := composeProjectName(target.ContainerName, target.DeploymentId)

	ctx, cancel := context.WithTimeout(context.Background(), healthCommandTimeout)
	defer cancel()

	var cmd *exec.Cmd
	if strings.TrimSpace(target.Service) == "" {
		cred, err := unprivilegedCredential()
		if err != nil {
			logger.DeployLogger.Printf("Infra health dep=%s skipped: no unprivileged user for host command: %v", target.DeploymentId, err)
			if depLog != nil {
				depLog.Printf("Health check skipped: host user 'nobody' unavailable: %v", err)
			}
			return
		}
		cmd = exec.CommandContext(ctx, "sh", "-c", target.Command)
		cmd.Dir = "/"
		cmd.SysProcAttr = &syscall.SysProcAttr{Credential: cred}
	} else {
		cmd = exec.CommandContext(
			ctx,
			"docker", "compose", "-p", project,
			"exec", "-T", target.Service,
			"sh", "-c", target.Command,
		)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.DeployLogger.Printf("Infra health dep=%s project=%s unhealthy: %v", target.DeploymentId, project, err)
		if depLog != nil {
			depLog.Printf("Health check unhealthy: %v\n%s", err, strings.TrimSpace(string(out)))
		}
		return
	}

	if err := api.ReportInfraHealth(target); err != nil {
		logger.DeployLogger.Printf("Infra health report dep=%s failed: %v", target.DeploymentId, err)
	}
}
