package docker

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	ctypes "github.com/pthsarmah/forge-agent/types"
)

func Deploy(dep ctypes.Deployment, projectPath string, framework string) error {

	//get dockerfile string
	var template string
	switch framework {
	case "Next.js":
		template = CreateNextJSDockerfile()
	default:
		template = ""
	}

	file, err := os.OpenFile(projectPath+"/Dockerfile.temp", os.O_CREATE, 0664)
	if err != nil {
		return fmt.Errorf("Could not create Dockerfile: %v\n", err)
	}

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

		//run
		cmd = exec.Command("docker", "run", "-p", "5555:3000", fmt.Sprintf("%s:%s", imageName, versionNo))

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err = cmd.Run(); err != nil {
			return fmt.Errorf("Could not run command: %v\n", err)
		}
	}

	//delete dockerfile
	err = os.Remove(projectPath + "/Dockerfile.temp")
	if err != nil {
		return fmt.Errorf("Could not delete dockerfile: %v", err)
	}

	fmt.Print("Deployed!")
	return nil
}
