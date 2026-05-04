package ctypes

type NodeInfo struct {
	UserId    string `json:"userId"`
	NodeId    string `json:"nodeId"`
	NodeToken string `json:"nodeToken"`
}

type Config struct {
	Nodes map[string]NodeInfo `json:"nodes"`
}
