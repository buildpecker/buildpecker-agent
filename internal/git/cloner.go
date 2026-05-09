package git

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
)

func CloneRepo(repoUrl string) (string, error) {

	repo := path.Base(repoUrl)
	path := strings.TrimSuffix(repo, ".git")

	dir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("Failed to fetch homedir %v", err)
	}
	projectDir := dir + "/forge"
	if err = os.MkdirAll(projectDir, 0755); err != nil {
		return "", fmt.Errorf("Failed to create directory %v", err)
	}
	pathDir := projectDir + "/" + path
	_, err = os.Stat(pathDir)
	if err == nil {
		return pathDir, fmt.Errorf("Repository already exists in path. Not cloning...\n")
	}

	cmd := exec.Command("git", "clone", repoUrl, pathDir)

	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("Failed to clone repo %s: %v\n", repoUrl, err)
	}

	fmt.Print("Repository cloned successfully!\n")
	return pathDir, nil
}
