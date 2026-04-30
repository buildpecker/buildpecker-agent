package api

type User struct {
	Name       string
	ExternalId string
	AuthToken  string
}

type Project struct {
	Name string
}

type Node struct {
	Name     string
	Cpu      string
	Memory   string
	Hostname string
	Ip       string
}

type Deployment struct {
	Name      string
	NodeId    string
	ProjectId string
	GithubUrl string
}

type Environment struct {
	Name         string
	DeploymentId string
	Variables    map[string]string
}
