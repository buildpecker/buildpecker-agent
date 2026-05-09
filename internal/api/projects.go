package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	ctypes "github.com/pthsarmah/forge-agent/types"
)

func SetProjectFramework(dep ctypes.Deployment, framework string) error {
	baseURL := strings.TrimRight(os.Getenv("CONVEX_SITE_URL"), "/")
	if baseURL == "" {
		return fmt.Errorf("CONVEX_SITE_URL is empty")
	}
	url := baseURL + "/projects/framework"

	body, err := json.Marshal(map[string]any{
		"id":        dep.Project.Id,
		"framework": framework,
	})
	if err != nil {
		return fmt.Errorf("marshal body: %w", err)
	}

	req, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+dep.NodeToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	res, err := (&http.Client{}).Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer res.Body.Close()

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if res.StatusCode >= 400 {
		return fmt.Errorf("http %d: %s", res.StatusCode, string(raw))
	}

	return nil
}
