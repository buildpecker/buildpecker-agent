package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	ctypes "github.com/pthsarmah/forge-agent/types"
	"io"
	"net/http"
	"os"
	"strings"
)

func GetEnvironmentSecrets(dep ctypes.Deployment) ([]ctypes.EnvVar, error) {
	baseURL := strings.TrimRight(os.Getenv("CONVEX_SITE_URL"), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("CONVEX_SITE_URL is empty")
	}
	url := baseURL + "/environments/secrets"

	body, err := json.Marshal(map[string]any{
		"projectId": dep.Project.Id,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal body: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+dep.NodeToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	res, err := (&http.Client{}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer res.Body.Close()

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("http %d: %s", res.StatusCode, string(raw))
	}

	var envMap []ctypes.EnvVar
	if err := json.Unmarshal(raw, &envMap); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return envMap, nil
}
