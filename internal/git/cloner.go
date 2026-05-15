package git

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"

	"github.com/pthsarmah/forge-agent/utils"
)

// progressLine matches transient "<phase>: NN% (x/y)" updates git prints on \r.
var progressLine = regexp.MustCompile(`\b(\d{1,3})%`)

// scanCRLF splits on \n or \r so git's carriage-return progress ticks become
// discrete tokens instead of one buffered blob.
func scanCRLF(data []byte, atEOF bool) (int, []byte, error) {
	for i, b := range data {
		if b == '\n' || b == '\r' {
			return i + 1, data[:i], nil
		}
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}

func CloneRepo(repoUrl string, deploymentID string) (string, error) {
	logger, _ := utils.GetLoggerInstance()
	var depLog *log.Logger
	if logger != nil {
		depLog, _ = logger.GetDeploymentLogger(deploymentID)
	}

	logf := func(format string, args ...any) {
		msg := fmt.Sprintf(format, args...)
		if logger != nil {
			logger.DeployLogger.Printf("dep=%s %s", deploymentID, msg)
		}
		if depLog != nil {
			depLog.Print(msg)
		}
	}

	repo := path.Base(repoUrl)
	path := strings.ToLower(strings.TrimSuffix(repo, ".git"))

	dir, err := os.UserHomeDir()
	if err != nil {
		logf("Clone failed: could not fetch homedir: %v", err)
		return "", fmt.Errorf("Failed to fetch homedir %v", err)
	}
	projectDir := dir + "/forge"
	if err = os.MkdirAll(projectDir, 0755); err != nil {
		logf("Clone failed: could not create directory: %v", err)
		return "", fmt.Errorf("Failed to create directory %v", err)
	}
	pathDir := projectDir + "/" + path
	_, err = os.Stat(pathDir)
	if err == nil {
		logf("Repository already exists at %s, not cloning", pathDir)
		return pathDir, fmt.Errorf("Repository already exists in path. Not cloning...\n")
	}

	logf("Cloning %s into %s", repoUrl, pathDir)

	cmd := exec.Command("git", "clone", "--progress", repoUrl, pathDir)

	pipe, err := cmd.StdoutPipe()
	if err != nil {
		logf("Clone failed: stdout pipe: %v", err)
		return "", fmt.Errorf("Failed to clone repo %s: %v\n", repoUrl, err)
	}
	cmd.Stderr = cmd.Stdout // git writes clone progress to stderr

	if err := cmd.Start(); err != nil {
		logf("Clone failed: start: %v", err)
		return "", fmt.Errorf("Failed to clone repo %s: %v\n", repoUrl, err)
	}

	sc := bufio.NewScanner(pipe)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	sc.Split(scanCRLF)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), " \t")
		if line == "" {
			continue
		}
		// collapse progress spam: skip interim "NN%" ticks, keep only the
		// final state of a phase (100% / "done.") and non-progress output.
		if m := progressLine.FindStringSubmatch(line); m != nil {
			if m[1] != "100" && !strings.Contains(line, "done.") {
				continue
			}
		}
		logf("%s", line)
	}

	if err := cmd.Wait(); err != nil {
		logf("Clone failed: %v", err)
		return "", fmt.Errorf("Failed to clone repo %s: %v\n", repoUrl, err)
	}

	logf("Repository cloned successfully")
	return pathDir, nil
}
