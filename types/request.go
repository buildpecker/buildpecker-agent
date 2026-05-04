package ctypes

type APIEnvelope struct {
	Status string `json:"status"`
}

type APISuccessResponse struct {
	Status   string         `json:"status"`
	Value    map[string]any `json:"value"`
	LogLines []string       `json:"logLines"`
}

type APIErrorResponse struct {
	Status       string         `json:"status"`
	ErrorMessage string         `json:"errorMessage"`
	ErrorData    map[string]any `json:"errorData"`
	LogLines     []string       `json:"logLines"`
}

type ConvexRequestBody struct {
	Path   string         `json:"path"`
	Args   map[string]any `json:"args"`
	Format string         `json:"format"`
}
