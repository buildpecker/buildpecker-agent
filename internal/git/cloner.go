package git

import (
	"fmt"
	"os"
	"os/exec"
)

func CloneRepo(repoUrl string, path string) {
	dir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to fetch homedir %v", err)
	}
	projectDir := dir + "/forge"
	if err = os.MkdirAll(projectDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create directory %v", err)
	}
	pathDir := projectDir + "/" + path
	_, err = os.Stat(pathDir)
	if err == nil {
		fmt.Fprintf(os.Stderr, "Repository already exists in path. Not cloning...")
	}

	cmd := exec.Command("git", "clone", repoUrl, pathDir)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to clone repo %s: %v", repoUrl, err)
		return
	}

	fmt.Print("Repository cloned successfully!")
}
