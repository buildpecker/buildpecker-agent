package deploy

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	ctypes "github.com/pthsarmah/forge-agent/types"
	"github.com/pthsarmah/forge-agent/utils"
)

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

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		logger.DeployLogger.Printf("Nixpack build failed dep=%s: %v", dep.Id, err)
		if depLog != nil {
			depLog.Printf("Nixpack build failed: %v", err)
		}
		return fmt.Errorf("could not run command: %w\n", err)
	}

	//build and run
	args := []string{
		"run",
		"-d",
		"-p", "5555:3000",
		"--name", fmt.Sprintf("%s", imageName),
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

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		logger.DeployLogger.Printf("Docker run failed dep=%s: %v", dep.Id, err)
		if depLog != nil {
			depLog.Printf("Docker run failed: %v", err)
		}
		return fmt.Errorf("could not run command: %w\n", err)
	}

	logger.DeployLogger.Printf("Container running dep=%s image=%s", dep.Id, imageName)
	if depLog != nil {
		depLog.Printf("Container running image=%s", imageName)
	}

	return nil
}
