package deploy

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	ctypes "github.com/pthsarmah/buildpecker-agent/types"
	"github.com/pthsarmah/buildpecker-agent/utils"
	"gopkg.in/yaml.v3"
)

var invalidProjectChars = regexp.MustCompile(`[^a-z0-9_-]+`)

func composeProjectName(containerName, depId string) string {
	name := strings.ToLower(strings.TrimSpace(containerName))
	name = invalidProjectChars.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-_")
	if name == "" {
		name = "infra-" + strings.ToLower(depId)
	}
	first := name[0]
	if !((first >= 'a' && first <= 'z') || (first >= '0' && first <= '9')) {
		name = "x" + name
	}
	return name
}

func mergeEnvFile(existing any, path string) []any {
	var files []any
	switch v := existing.(type) {
	case string:
		files = append(files, v)
	case []any:
		files = append(files, v...)
	}
	files = append(files, path)
	return files
}

// transformCompose strips explicit container_name entries (so every deployment
// is namespaced solely by its compose project) and force-injects the secrets
// env file into every service.
func transformCompose(composeYaml, envFilePath string) (string, error) {
	var root map[string]any
	if err := yaml.Unmarshal([]byte(composeYaml), &root); err != nil {
		return "", fmt.Errorf("invalid compose yaml: %w", err)
	}

	services, ok := root["services"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("compose yaml has no services map")
	}

	for name, raw := range services {
		svc, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		delete(svc, "container_name")
		svc["env_file"] = mergeEnvFile(svc["env_file"], envFilePath)
		services[name] = svc
	}

	out, err := yaml.Marshal(root)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func writeServiceConfig(dir string, fileName string, fileContents string) error {
	serviceConfig := filepath.Join(dir, fileName)
	if err := os.WriteFile(serviceConfig, []byte(fileContents), 0644); err != nil {
		os.RemoveAll(dir)
		return err
	}
	return nil
}

func writeComposeProject(dep ctypes.Deployment, envs []ctypes.EnvVar) (string, error) {
	dir, err := os.MkdirTemp("", "buildpecker-infra-*")
	if err != nil {
		return "", err
	}

	envFilePath := filepath.Join(dir, ".env")
	var b strings.Builder
	for _, e := range envs {
		b.WriteString(e.Key)
		b.WriteString("=")
		b.WriteString(e.Value)
		b.WriteString("\n")
	}
	if err := os.WriteFile(envFilePath, []byte(b.String()), 0600); err != nil {
		os.RemoveAll(dir)
		return "", err
	}

	compose, err := transformCompose(dep.Infra.ComposeYaml, envFilePath)
	if err != nil {
		os.RemoveAll(dir)
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte(compose), 0644); err != nil {
		os.RemoveAll(dir)
		return "", err
	}

	if dep.Infra.ConfigFileName != "" {
		if err := writeServiceConfig(dir, dep.Infra.ConfigFileName, dep.Infra.Config); err != nil {
			return "", err
		}
	}

	return dir, nil
}

type composePublisher struct {
	TargetPort    int `json:"TargetPort"`
	PublishedPort int `json:"PublishedPort"`
}

type composePs struct {
	Publishers []composePublisher `json:"Publishers"`
}

// discoverPortMap reports the container-port -> published-host-port mapping for
// every service in the project, so Cloudflare ingress can be wired per route.
func discoverPortMap(project string) []ctypes.PortMapEntry {
	out, err := exec.Command("docker", "compose", "-p", project, "ps", "--format", "json").Output()
	if err != nil {
		return nil
	}

	seen := map[int]bool{}
	var entries []ctypes.PortMapEntry
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var ps composePs
		if err := json.Unmarshal([]byte(line), &ps); err != nil {
			continue
		}
		for _, p := range ps.Publishers {
			if p.TargetPort > 0 && p.PublishedPort > 0 && !seen[p.TargetPort] {
				seen[p.TargetPort] = true
				entries = append(entries, ctypes.PortMapEntry{
					ContainerPort: p.TargetPort,
					PublishedPort: p.PublishedPort,
				})
			}
		}
	}
	return entries
}

func InfraDeploy(dep ctypes.Deployment, envs []ctypes.EnvVar) ([]ctypes.PortMapEntry, error) {
	logger, _ := utils.GetLoggerInstance()
	depLog, _ := logger.GetDeploymentLogger(dep.Id)

	project := composeProjectName(dep.Infra.ContainerName, dep.Id)

	dir, err := writeComposeProject(dep, envs)
	if err != nil {
		return nil, fmt.Errorf("could not stage compose project: %w", err)
	}
	defer os.RemoveAll(dir)

	logger.DeployLogger.Printf("Infra compose up dep=%s project=%s template=%s", dep.Id, project, dep.Infra.Template.Identifier)
	if depLog != nil {
		depLog.Printf("Infra compose up project=%s template=%s", project, dep.Infra.Template.Identifier)
	}

	args := []string{
		"compose",
		"-p", project,
		"--project-directory", dir,
		"-f", filepath.Join(dir, "docker-compose.yml"),
		"up", "-d", "--remove-orphans",
	}

	cmd := exec.Command("docker", args...)
	if err := runStreaming(cmd, logger.DeployLogger, depLog); err != nil {
		return nil, fmt.Errorf("docker compose up failed: %w", err)
	}

	var portMap []ctypes.PortMapEntry
	if len(dep.Routes) > 0 {
		portMap = discoverPortMap(project)
		if len(portMap) == 0 {
			logger.DeployLogger.Printf("Infra public requested but no published ports found dep=%s project=%s", dep.Id, project)
			if depLog != nil {
				depLog.Println("Public requested but no published ports found; skipping ingress")
			}
		}
	}

	logger.DeployLogger.Printf("Infra running dep=%s project=%s ports=%v", dep.Id, project, portMap)
	if depLog != nil {
		depLog.Printf("Infra running project=%s ports=%v", project, portMap)
	}
	return portMap, nil
}

func InfraDelete(dep ctypes.Deployment) error {
	logger, _ := utils.GetLoggerInstance()

	project := composeProjectName(dep.Infra.ContainerName, dep.Id)
	logger.DeployLogger.Printf("Infra compose down dep=%s project=%s", dep.Id, project)

	cmd := exec.Command("docker", "compose", "-p", project, "down", "--remove-orphans")
	if out, err := cmd.CombinedOutput(); err != nil {
		logger.DeployLogger.Printf("docker compose down dep=%s project=%s failed: %v output=%s",
			dep.Id, project, err, strings.TrimSpace(string(out)))
		return err
	}
	logger.DeployLogger.Printf("docker compose down dep=%s project=%s ok", dep.Id, project)
	return nil
}
