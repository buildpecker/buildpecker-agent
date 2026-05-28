package config

import _ "embed"

//go:embed nixpacks.vite.toml
var NixpacksViteToml string

//go:embed nixpacks.svelte-node.toml
var NixpacksSvelteNodeToml string
