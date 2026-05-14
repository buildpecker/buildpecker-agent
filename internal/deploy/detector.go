package deploy

import (
	"fmt"
	"os"
	"strings"

	ctypes "github.com/pthsarmah/forge-agent/types"
	"github.com/pthsarmah/forge-agent/utils"
)

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
			strContents := string(contents)

			//Next.js
			if strings.Contains(strContents, "next dev") ||
				strings.Contains(strContents, "next start") {
				return "Next.js", nil
			}
		}
	}

	return "Unknown", nil
}
