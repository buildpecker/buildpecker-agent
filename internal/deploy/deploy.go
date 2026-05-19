package deploy

import (
	"bufio"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"strings"

	ctypes "github.com/pthsarmah/forge-agent/types"
	"github.com/pthsarmah/forge-agent/utils"
)

// runStreaming starts cmd and fans every stdout/stderr line, live, into each
// non-nil sink. Replaces CombinedOutput so a long build is tailable per
// deployment instead of dumped once on exit.
func runStreaming(cmd *exec.Cmd, sinks ...*log.Logger) error {
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout // fold stderr into the same pipe
	if err := cmd.Start(); err != nil {
		return err
	}
	sc := bufio.NewScanner(pipe)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		for _, s := range sinks {
			if s != nil {
				s.Print(line)
			}
		}
	}
	return cmd.Wait()
}

var nixpackEnvs = map[string]string{
	"NIXPACKS_NODE_VERSION": "22",
}

func NixpackDeploy(dep ctypes.Deployment, envs []ctypes.EnvVar, projectPath string, framework string) error {
	logger, _ := utils.GetLoggerInstance()
	depLog, _ := logger.GetDeploymentLogger(dep.Id)

	imageName := strings.TrimSuffix(path.Base(projectPath), ".git")
	versionNo := "v1"

	logger.DeployLogger.Printf("Nixpack build start dep=%s image=%s:%s framework=%s", dep.Id, imageName, versionNo, framework)
	if depLog != nil {
		depLog.Printf("Nixpack build start image=%s:%s framework=%s", imageName, versionNo, framework)
	}

	//nixpack build
	nixargs := []string{
		"build", projectPath,
		"--name", fmt.Sprintf("%s:%s", imageName, versionNo),
	}

	if pkgs := DetectNativePkgs(projectPath); len(pkgs) > 0 {
		logger.DeployLogger.Printf("Injecting native build pkgs dep=%s pkgs=%v", dep.Id, pkgs)
		if depLog != nil {
			depLog.Printf("Injecting native build pkgs: %v", pkgs)
		}
		nixargs = append(nixargs, "--pkgs", strings.Join(pkgs, " "))
	}

	cmd := exec.Command("nixpacks", nixargs...)

	cmd.Env = os.Environ()
	for k, v := range nixpackEnvs {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	if err := runStreaming(cmd, logger.DeployLogger, depLog); err != nil {
		logger.DeployLogger.Printf("Nixpack build failed dep=%s: %v", dep.Id, err)
		if depLog != nil {
			depLog.Printf("Nixpack build failed: %v", err)
		}
		return fmt.Errorf("could not run command: %w", err)
	}

	sanitizedId := strings.ReplaceAll(dep.Id, "_", "-") // routers must be DNS-safe

	args := []string{
		"run",
		"-d",
		"--network", "forge",
		"--name", imageName,
		"--label", "traefik.enable=true",
		"--label", "traefik.docker.network=forge",
		"--label", fmt.Sprintf("traefik.http.routers.%s.rule=Host(`%s.parthajeet.xyz`)",
			sanitizedId, sanitizedId),
		"--label", fmt.Sprintf("traefik.http.routers.%s.entrypoints=websecure", sanitizedId),
		"--label", fmt.Sprintf("traefik.http.routers.%s.tls=true", sanitizedId),
		"--label", fmt.Sprintf("traefik.http.routers.%s.tls.certresolver=le", sanitizedId),
		"--label", fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port=3000",
			sanitizedId),
	}

	for _, e := range envs {
		args = append(args,
			"--env",
			e.Key+"="+e.Value,
		)
	}

	args = append(args,
		fmt.Sprintf("%s:%s", imageName, versionNo),
	)

	cmd = exec.Command("docker", args...)

	if err := runStreaming(cmd, logger.DeployLogger, depLog); err != nil {
		logger.DeployLogger.Printf("Docker run failed dep=%s: %v", dep.Id, err)
		if depLog != nil {
			depLog.Printf("Docker run failed: %v", err)
		}
		return fmt.Errorf("could not run command: %w", err)
	}

	logger.DeployLogger.Printf("Container running dep=%s image=%s", dep.Id, imageName)
	if depLog != nil {
		depLog.Printf("Container running image=%s", imageName)
	}

	return nil
}
