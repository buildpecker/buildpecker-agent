package ctypes

type FrameworkInfo struct {
	Id           string
	DisplayName  string
	DefaultPort  int
	BuildFolder  string
	StaticBuild  bool
	AddPackages  []string
	NixpacksToml string
}

type Framework string
