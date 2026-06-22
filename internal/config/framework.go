package config

import (
	_ "embed"
	ctypes "github.com/pthsarmah/buildpecker-agent/types"
)

const (
	FrameworkNextJS          ctypes.Framework = "nextjs"
	FrameworkVite            ctypes.Framework = "vite"
	FrameworkSvelte          ctypes.Framework = "svelte"
	FrameworkSvelteKitStatic ctypes.Framework = "sveltekit-static-adapter"
	FrameworkSvelteKitNode   ctypes.Framework = "sveltekit-node-adapter"
)

var frameworkInfos = map[ctypes.Framework]ctypes.FrameworkInfo{
	FrameworkNextJS: {
		Id:                "nextjs",
		DisplayName:       "Next.js",
		DefaultPort:       3000,
		BuildFolder:       ".next",
		StaticBuild:       false,
		PublicEnvPrefixes: []string{"NEXT_PUBLIC_"},
	},
	FrameworkVite: {
		Id:                "vite",
		DisplayName:       "Vite",
		DefaultPort:       3000,
		BuildFolder:       "dist",
		StaticBuild:       true,
		PublicEnvPrefixes: []string{"VITE_"},
	},
	FrameworkSvelte: {
		Id:                "svelte",
		DisplayName:       "Svelte",
		DefaultPort:       80,
		BuildFolder:       "dist",
		StaticBuild:       true,
		NixpacksToml:      NixpacksViteToml,
		PublicEnvPrefixes: []string{"VITE_", "PUBLIC_"},
	},
	FrameworkSvelteKitStatic: {
		Id:                "svelte-kit-static",
		DisplayName:       "SvelteKit (Static Adapter)",
		DefaultPort:       80,
		BuildFolder:       "dist",
		StaticBuild:       true,
		NixpacksToml:      NixpacksViteToml,
		PublicEnvPrefixes: []string{"VITE_", "PUBLIC_"},
	},
	FrameworkSvelteKitNode: {
		Id:                "svelte-kit-node",
		DisplayName:       "SvelteKit (Node Adapter)",
		DefaultPort:       3000,
		BuildFolder:       "dist",
		StaticBuild:       false,
		NixpacksToml:      NixpacksSvelteNodeToml,
		PublicEnvPrefixes: []string{"VITE_", "PUBLIC_"},
	},
}

func GetFrameworkInfo(fw string) ctypes.FrameworkInfo {
	return frameworkInfos[ctypes.Framework(fw)]
}

func GetFrameworkInfoByDisplayName(dName string) ctypes.FrameworkInfo {
	for _, info := range frameworkInfos {
		if info.DisplayName == dName {
			return info
		}
	}
	return ctypes.FrameworkInfo{}
}
