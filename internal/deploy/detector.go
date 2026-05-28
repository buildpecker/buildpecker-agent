package deploy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pthsarmah/forge-agent/internal/config"
	ctypes "github.com/pthsarmah/forge-agent/types"
	"github.com/pthsarmah/forge-agent/utils"
)

type PackageJSON struct {
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

func DetectFramework(dep ctypes.Deployment, path string) (ctypes.FrameworkInfo, error) {
	logger, _ := utils.GetLoggerInstance()

	//	if dep.Project.Framework != "Unknown" {
	//		logger.DeployLogger.Printf("Using cached framework dep=%s framework=%s", dep.Id, dep.Project.Framework)
	//
	//		framework := config.GetFrameworkInfoByDisplayName(dep.Project.Framework)
	//		return framework, nil
	//	}

	pkgIds := []string{"package.json", "go.mod", "requirements.txt"}
	var pkgFilePath string

	for _, file := range pkgIds {
		_, err := os.Stat(path + "/" + file)
		if err == nil {
			pkgFilePath = path + "/" + file
			contents, err := os.ReadFile(pkgFilePath)
			if err != nil {
				logger.DeployLogger.Printf("Read manifest %s failed dep=%s: %v", pkgFilePath, dep.Id, err)
				return ctypes.FrameworkInfo{}, fmt.Errorf("Error in reading file: %v", err)
			}

			switch file {
			case "package.json":
				var pkg PackageJSON
				if err := json.Unmarshal(contents, &pkg); err != nil {
					return ctypes.FrameworkInfo{}, err
				}

				var framework ctypes.FrameworkInfo
				detectors := []func(string, PackageJSON) (ctypes.FrameworkInfo, bool, error){
					detectNextJS,
					detectSvelteKit,
					detectAstro,
					detectRemix,
					detectVite,
				}

				for _, detector := range detectors {
					fw, isFw, err := detector(path, pkg)
					if err != nil {
						return ctypes.FrameworkInfo{}, err
					}
					if isFw {
						framework = fw
						break
					}
				}

				return framework, nil
			default:
				return ctypes.FrameworkInfo{}, nil
			}
		}
	}

	return ctypes.FrameworkInfo{}, nil
}

func hasAnyFile(path string, files ...string) (bool, error) {
	for _, file := range files {
		_, err := os.Stat(filepath.Join(path, file))
		switch {
		case err == nil:
			return true, nil
		case os.IsNotExist(err):
			continue
		default:
			return false, err
		}
	}

	return false, nil
}

func hasDependency(pkg PackageJSON, dep string) bool {
	if _, ok := pkg.Dependencies[dep]; ok {
		return true
	}

	if _, ok := pkg.DevDependencies[dep]; ok {
		return true
	}

	return false
}

func hasScriptContaining(pkg PackageJSON, patterns ...string) bool {
	for _, script := range pkg.Scripts {
		for _, pattern := range patterns {
			if strings.Contains(script, pattern) {
				return true
			}
		}
	}
	return false
}

func detectNextJS(path string, pkg PackageJSON) (ctypes.FrameworkInfo, bool, error) {
	hasConfig, err := hasAnyFile(path,
		"next.config.js",
		"next.config.ts",
		"next.config.mjs",
		"next.config.cjs",
	)
	if err != nil {
		return ctypes.FrameworkInfo{}, false, err
	}

	hasDep := hasDependency(pkg, "next")

	hasScript := hasScriptContaining(pkg,
		"next",
	)

	if hasConfig && hasDep && hasScript {
		return config.GetFrameworkInfo(string(config.FrameworkNextJS)), true, nil
	}

	return ctypes.FrameworkInfo{}, false, nil
}

func detectVite(path string, pkg PackageJSON) (ctypes.FrameworkInfo, bool, error) {
	hasConfig, err := hasAnyFile(path,
		"vite.config.js",
		"vite.config.ts",
		"vite.config.mjs",
		"vite.config.cjs",
	)
	if err != nil {
		return ctypes.FrameworkInfo{}, false, err
	}

	hasDep := hasDependency(pkg, "vite")

	hasScript := hasScriptContaining(pkg,
		"vite",
	)

	counter := 0
	switch {
	case hasConfig:
		counter++
		fallthrough
	case hasDep:
		counter++
		fallthrough
	case hasScript:
		counter++
	}

	if counter >= 2 {
		return config.GetFrameworkInfo(string(config.FrameworkVite)), true, nil
	}

	return ctypes.FrameworkInfo{}, false, nil
}

func detectAstro(path string, pkg PackageJSON) (ctypes.FrameworkInfo, bool, error) {
	hasConfig, err := hasAnyFile(path,
		"astro.config.js",
		"astro.config.ts",
		"astro.config.mjs",
		"astro.config.cjs",
	)
	if err != nil {
		return ctypes.FrameworkInfo{}, false, err
	}

	hasDep := hasDependency(pkg, "astro")

	hasScript := hasScriptContaining(pkg,
		"astro",
	)

	if hasConfig && hasDep && hasScript {
		return config.GetFrameworkInfo("astro"), true, nil
	}

	return ctypes.FrameworkInfo{}, false, nil
}

func detectSvelteKit(path string, pkg PackageJSON) (ctypes.FrameworkInfo, bool, error) {
	hasConfig, err := hasAnyFile(path,
		"svelte.config.js",
		"svelte.config.cjs",
	)
	if err != nil {
		return ctypes.FrameworkInfo{}, false, err
	}

	hasDep :=
		hasDependency(pkg, "@sveltejs/kit") ||
			hasDependency(pkg, "svelte-kit")

	hasStaticAdapter := hasDependency(pkg, "@sveltejs/adapter-static")
	hasNodeAdapter := hasDependency(pkg, "@sveltejs/adapter-node")
	hasAutoAdapter := hasDependency(pkg, "@sveltejs/adapter-auto")

	hasScript := hasScriptContaining(pkg,
		"svelte-kit",
		"vite",
	)

	if hasConfig && hasDep && hasScript {
		switch {
		case hasStaticAdapter:
			return config.GetFrameworkInfo(string(config.FrameworkSvelteKitStatic)), true, nil
		case hasNodeAdapter:
			return config.GetFrameworkInfo(string(config.FrameworkSvelteKitNode)), true, nil
		case hasAutoAdapter:
			return ctypes.FrameworkInfo{}, false, fmt.Errorf("[ERROR]: SvelteKit detected with auto-adapter. Please switch to an adapter-node or adapter-static configuration.")
		}
	}

	return ctypes.FrameworkInfo{}, false, nil
}

func detectRemix(path string, pkg PackageJSON) (ctypes.FrameworkInfo, bool, error) {
	hasConfig, err := hasAnyFile(path,
		"remix.config.js",
		"remix.config.cjs",
	)
	if err != nil {
		return ctypes.FrameworkInfo{}, false, err
	}

	hasDep :=
		hasDependency(pkg, "@remix-run/react") ||
			hasDependency(pkg, "@remix-run/node")

	hasScript := hasScriptContaining(pkg,
		"remix dev",
		"remix build",
		"remix-serve",
	)

	if hasConfig && hasDep && hasScript {
		return config.GetFrameworkInfo("remix"), true, nil
	}

	return ctypes.FrameworkInfo{}, false, nil
}
