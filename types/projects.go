package ctypes

type Project struct {
	Name          string `json:"name"`
	OwnerId       string `json:"ownerId"`
	Framework     string `json:"framework"`
	DefaultBranch string `json:"defaultBranch"`
	RepoUrl       string `json:"repoUrl"`
}
