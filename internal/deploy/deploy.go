package deploy

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	ctypes "github.com/pthsarmah/forge-agent/types"
)

var nixpackEnvs = map[string]string{
	"NIXPACKS_NODE_VERSION": "22",
}

func NixpackDeploy(dep ctypes.Deployment, envs []ctypes.EnvVar, projectPath string, framework string) error {

	imageName := strings.TrimSuffix(path.Base(projectPath), ".git")
	versionNo := "v1"

	//nixpack build
	nixargs := []string{
		"build", projectPath,
		"--name", fmt.Sprintf("%s:%s", imageName, versionNo),
	}

	if pkgs := DetectNativePkgs(projectPath); len(pkgs) > 0 {
		fmt.Printf("Injecting native build pkgs: %v\n", pkgs)
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
		return fmt.Errorf("could not run command: %w\n", err)
	}

	fmt.Print("Container running!\n")

	return nil
}
