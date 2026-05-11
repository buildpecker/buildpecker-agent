package docker

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	ctypes "github.com/pthsarmah/forge-agent/types"
)

func Deploy(dep ctypes.Deployment, envs []ctypes.EnvVar, projectPath string, framework string) error {

	//get dockerfile string
	var template string
	switch framework {
	case "Next.js":
		template = CreateNextJSDockerfile()
	default:
		template = ""
	}

	file, err := os.OpenFile(
		projectPath+"/Dockerfile.temp",
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
		0664,
	)
	if err != nil {
		return fmt.Errorf("Could not create Dockerfile: %v\n", err)
	}
	defer file.Close()

	imageName := strings.TrimSuffix(path.Base(projectPath), ".git")
	versionNo := "v1"

	//build and run
	if template != "" {
		//write
		_, err = file.Write([]byte(template))
		if err != nil {
			return fmt.Errorf("Could not write to Dockerfile: %v\n", err)
		}

		//build
		cmd := exec.Command("docker", "build", "-f", fmt.Sprintf("%s/%s", projectPath, "Dockerfile.temp"), projectPath, "-t", fmt.Sprintf("%s:%s", imageName, versionNo))

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err = cmd.Run(); err != nil {
			return fmt.Errorf("Could not run command: %v\n", err)
		}

		fmt.Print("Image built\n")

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

		if err = cmd.Run(); err != nil {
			return fmt.Errorf("could not run command: %w\n", err)
		}

		fmt.Print("Container running!\n")
	}

	//delete dockerfile
	err = os.Remove(projectPath + "/Dockerfile.temp")
	if err != nil {
		return fmt.Errorf("Could not delete dockerfile: %v\n", err)
	}

	fmt.Print("Deployed!\n")
	return nil
}
