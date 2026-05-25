package deploy

import (
	_ "embed"

	"github.com/pthsarmah/forge-agent/internal/config"
)

type FrameworkInfo struct {
	Port         int
	BuildFolder  string
	StaticBuild  bool
	AddPackages  []string
	NixpacksToml string
}

var frameworkInfos = map[string]FrameworkInfo{
	"Next.js": {
		Port:        3000,
		BuildFolder: ".next",
		StaticBuild: false,
	},
	"Vite": {
		Port:         80,
		BuildFolder:  "dist",
		StaticBuild:  true,
		NixpacksToml: config.NixpacksViteToml,
	},
}

func GetFrameworkInfo(fw string) FrameworkInfo {
	return frameworkInfos[fw]
}
