package ctypes

type NodeInfo struct {
	NodeId    string `json:"nodeId"`
	NodeToken string `json:"nodeToken"`
}

type Config struct {
	Nodes map[string]NodeInfo `json:"nodes"`
}
