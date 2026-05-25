package deploy

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
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
	"NIXPACKS_NODE_VERSION": "20",
}

func freeHostPort() (int, error) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func updateIgnoreFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	var lines []string
	hasUnignore := false

	for line := range strings.SplitSeq(string(data), "\n") {
		trimmed := strings.TrimSpace(line)

		switch trimmed {
		case ".nixpacks", ".nixpacks/", "/.nixpacks", "/.nixpacks/":
			// remove ignore rule
			continue
		case "!.nixpacks", "!/.nixpacks":
			hasUnignore = true
		}

		lines = append(lines, line)
	}

	if !hasUnignore {
		lines = append(lines,
			"!.nixpacks",
			"!.nixpacks/**",
		)
	}

	content := strings.Join(lines, "\n")
	return os.WriteFile(path, []byte(content), 0644)
}

func writeNixpacksConfig(cfg string) (string, error) {
	dir, err := os.MkdirTemp("", "nixpacks-*")
	if err != nil {
		return "", err
	}

	cfgPath := filepath.Join(dir, "nixpacks.toml")

	err = os.WriteFile(cfgPath, []byte(cfg), 0644)
	if err != nil {
		return "", err
	}

	return cfgPath, nil
}

func NixpackDeploy(dep ctypes.Deployment, envs []ctypes.EnvVar, projectPath string, framework string) (int, error) {

	//set .gitignore or .dockerignore flags to allow .nixpacks
	noGitPath := strings.TrimSuffix(projectPath, ".git")
	for _, name := range []string{".gitignore", ".dockerignore"} {
		path := filepath.Join(noGitPath, name)

		if err := updateIgnoreFile(path); err != nil {
			return 0, fmt.Errorf("could not update %s: %w", name, err)
		}
	}

	logger, _ := utils.GetLoggerInstance()
	depLog, _ := logger.GetDeploymentLogger(dep.Id)

	imageName := strings.TrimSuffix(path.Base(projectPath), ".git")
	versionNo := "v1"

	logger.DeployLogger.Printf("Nixpack build start dep=%s image=%s:%s framework=%s", dep.Id, imageName, versionNo, framework)
	if depLog != nil {
		depLog.Printf("Nixpack build start image=%s:%s framework=%s", imageName, versionNo, framework)
	}

	fwInfo := GetFrameworkInfo(framework)

	//nixpack build
	nixargs := []string{
		"build", projectPath,
		"--name", fmt.Sprintf("%s:%s", imageName, versionNo),
	}

	//custom toml if needed
	if fwInfo.NixpacksToml != "" {
		cfgPath, err := writeNixpacksConfig(fwInfo.NixpacksToml)
		if err != nil {
			return 0, err
		}

		nixargs = append(nixargs, "--config", cfgPath)
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
		return 0, fmt.Errorf("could not run command: %w", err)
	}

	hostPort, err := freeHostPort()
	if err != nil {
		logger.DeployLogger.Printf("Allocate host port failed dep=%s: %v", dep.Id, err)
		if depLog != nil {
			depLog.Printf("Allocate host port failed: %v", err)
		}
		return 0, fmt.Errorf("could not allocate host port: %w", err)
	}

	rmCmd := exec.Command("docker", "rm", "-f", imageName)
	if out, err := rmCmd.CombinedOutput(); err != nil {
		logger.DeployLogger.Printf("No previous container to remove dep=%s name=%s: %s",
			dep.Id, imageName, strings.TrimSpace(string(out)))
	} else {
		logger.DeployLogger.Printf("Removed previous container dep=%s name=%s", dep.Id, imageName)
		if depLog != nil {
			depLog.Printf("Removed previous container name=%s", imageName)
		}
	}

	args := []string{
		"run",
		"-d",
		"--name", imageName,
		"--restart", "unless-stopped",
		"-p", fmt.Sprintf("127.0.0.1:%d:%d", hostPort, frameworkInfos[framework].Port),
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
		return 0, fmt.Errorf("could not run command: %w", err)
	}

	logger.DeployLogger.Printf("Container running dep=%s image=%s port=%d", dep.Id, imageName, hostPort)
	if depLog != nil {
		depLog.Printf("Container running image=%s port=%d", imageName, hostPort)
	}

	return hostPort, nil
}
