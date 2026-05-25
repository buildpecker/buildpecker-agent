package deploy

import (
	//	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	ctypes "github.com/pthsarmah/forge-agent/types"
	"github.com/pthsarmah/forge-agent/utils"
)

type PackageJSON struct {
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

func DetectFramework(dep ctypes.Deployment, path string) (string, error) {
	logger, _ := utils.GetLoggerInstance()

	if dep.Project.Framework != "Unknown" {
		logger.DeployLogger.Printf("Using cached framework dep=%s framework=%s", dep.Id, dep.Project.Framework)
		return dep.Project.Framework, nil
	}

	pkgIds := []string{"package.json", "go.mod", "pom.xml"}
	var pkgFilePath string

	for _, file := range pkgIds {
		_, err := os.Stat(path + "/" + file)
		if err == nil {
			pkgFilePath = path + "/" + file
			contents, err := os.ReadFile(pkgFilePath)
			if err != nil {
				logger.DeployLogger.Printf("Read manifest %s failed dep=%s: %v", pkgFilePath, dep.Id, err)
				return "", fmt.Errorf("Error in reading file: %v", err)
			}

			switch file {
			case "package.json":
				//Next.js
				isNextJS, err := detectNextJS(path, contents)
				if err != nil {
					return "", err
				}
				//Vitejs
				isVite, err := detectVite(path, contents)
				if err != nil {
					return "", err
				}
				//Angular

				var frameworks []string

				if isNextJS {
					frameworks = append(frameworks, "Next.js")
				}
				if isVite {
					frameworks = append(frameworks, "Vite")
				}

				return strings.Join(frameworks, ", "), nil
			default:
				return "Unknown", nil
			}
		}
	}

	return "Unknown", nil
}

func detectNextJS(path string, rawContents []byte) (bool, error) {
	nextConfigs := []string{
		"next.config.ts",
		"next.config.js",
	}

	found := false
	for _, file := range nextConfigs {
		if _, err := os.Stat(filepath.Join(path, file)); err == nil {
			found = true
			break
		}
	}

	if !found {
		return false, nil
	}

	//	strContents := string(rawContents)
	//	if !strings.Contains(strContents, "next dev") && !strings.Contains(strContents, "next build") {
	//		return false, nil
	//	}
	//
	//	var packageJson PackageJSON
	//
	//	if err := json.Unmarshal(rawContents, &packageJson); err != nil {
	//		return false, fmt.Errorf("Unmarshal error: %v", err)
	//	}
	//
	//	_, ok := packageJson.Dependencies["next"]
	//	if !ok {
	//		return false, nil
	//	}

	return true, nil
}

func detectVite(path string, rawContents []byte) (bool, error) {
	viteConfigs := []string{
		"vite.config.ts",
		"vite.config.js",
	}

	found := false
	for _, file := range viteConfigs {
		if _, err := os.Stat(filepath.Join(path, file)); err == nil {
			found = true
			break
		}
	}

	if !found {
		return false, nil
	}

	//	strContents := string(rawContents)
	//	if !strings.Contains(strContents, "vite build") {
	//		return false, nil
	//	}
	//
	//	var packageJson PackageJSON
	//
	//	if err := json.Unmarshal(rawContents, &packageJson); err != nil {
	//		return false, fmt.Errorf("Unmarshal error: %v", err)
	//	}
	//
	//	_, ok := packageJson.DevDependencies["vite"]
	//	if !ok {
	//		return false, nil
	//	}

	return true, nil
}
