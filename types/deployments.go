package ctypes

type Job struct {
	Action string
	Data   any
}

type Deployment struct {
	Id           string       `json:"_id"`
	CreationTime float64      `json:"_creationTime"`
	Status       string       `json:"status"`
	Type         string       `json:"type"`
	Name         string       `json:"name"`
	NodeId       string       `json:"nodeId"`
	NodeToken    string       `json:"nodeToken,omitempty"`
	ProjectId    string       `json:"projectId"`
	PublicUrl    string       `json:"publicUrl"`
	ImageUri     string       `json:"imageUri"`
	Branch       string       `json:"branch"`
	Sha          string       `json:"sha"`
	HealthToken  string       `json:"healthToken,omitempty"`
	Project      Project      `json:"project"`
	Infra        Infra        `json:"infra"`
	Routes       []InfraRoute `json:"routes"`
}

type InfraTemplate struct {
	Id         string `json:"_id"`
	Identifier string `json:"identifier"`
	Name       string `json:"name"`
	Version    string `json:"version"`
}

type Infra struct {
	Id            string        `json:"_id"`
	ContainerName string        `json:"containerName"`
	ComposeYaml   string        `json:"composeYaml"`
	Template      InfraTemplate `json:"template"`
}

type InfraRoute struct {
	Name          string `json:"name"`
	Hostname      string `json:"hostname"`
	ContainerPort int    `json:"containerPort"`
}

type PortMapEntry struct {
	ContainerPort int `json:"containerPort"`
	PublishedPort int `json:"publishedPort"`
}

type PostInstallRun struct {
	Id            string `json:"_id"`
	DeploymentId  string `json:"deploymentId"`
	Name          string `json:"name"`
	Service       string `json:"service"`
	Command       string `json:"command"`
	ContainerName string `json:"containerName"`
	NodeToken     string `json:"nodeToken,omitempty"`
}
