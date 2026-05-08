package ctypes

type Job struct {
	Action string
	Data   any
}

type Deployment struct {
	Id           string  `json:"_id"`
	CreationTime float64 `json:"_creationTime"`
	Status       string  `json:"status"`
	Name         string  `json:"name"`
	NodeId       string  `json:"nodeId"`
	NodeToken    string  `json:"nodeToken,omitempty"`
	ProjectId    string  `json:"projectId"`
	ImageUri     string  `json:"imageUri"`
	Branch       string  `json:"branch"`
	Sha          string  `json:"sha"`
	Project      Project `json:"project"`
}
