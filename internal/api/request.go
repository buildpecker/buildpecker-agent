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

// -1 - other failure (connection reset, transport failure)
// 0 - http failure
// 1 - success

func Post(
	path string,
	contentType string,
	data ctypes.ConvexRequestBody,
	headers map[string]string,
) (int, ctypes.APISuccessResponse, ctypes.APIErrorResponse, error) {
	baseURL := strings.TrimRight(os.Getenv("CONVEX_CLOUD_URL"), "/")
	if baseURL == "" {
		return -1, ctypes.APISuccessResponse{}, ctypes.APIErrorResponse{}, fmt.Errorf("CONVEX_CLOUD_URL is empty")
	}

	url := baseURL + "/" + strings.TrimLeft(path, "/")

	body, err := json.Marshal(data)
	if err != nil {
		return -1, ctypes.APISuccessResponse{}, ctypes.APIErrorResponse{}, fmt.Errorf("marshal request body: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return -1, ctypes.APISuccessResponse{}, ctypes.APIErrorResponse{}, fmt.Errorf("create request: %w", err)
	}

	if contentType == "" {
		contentType = "application/json"
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "application/json")

	for key, val := range headers {
		req.Header.Set(key, val)
	}

	res, err := (&http.Client{}).Do(req)
	if err != nil {
		return -1, ctypes.APISuccessResponse{}, ctypes.APIErrorResponse{}, fmt.Errorf("execute request: %w", err)
	}
	defer res.Body.Close()

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return -1, ctypes.APISuccessResponse{}, ctypes.APIErrorResponse{}, fmt.Errorf("read response body: %w", err)
	}

	var env ctypes.APIEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return -1, ctypes.APISuccessResponse{}, ctypes.APIErrorResponse{}, fmt.Errorf("decode response envelope: %w", err)
	}

	switch env.Status {
	case "success":
		var ok ctypes.APISuccessResponse
		if err := json.Unmarshal(raw, &ok); err != nil {
			return -1, ctypes.APISuccessResponse{}, ctypes.APIErrorResponse{}, fmt.Errorf("decode success response: %w", err)
		}
		return 1, ok, ctypes.APIErrorResponse{}, nil

	case "error":
		var apiErr ctypes.APIErrorResponse
		if err := json.Unmarshal(raw, &apiErr); err != nil {
			return -1, ctypes.APISuccessResponse{}, ctypes.APIErrorResponse{}, fmt.Errorf("decode error response: %w", err)
		}
		return 0, ctypes.APISuccessResponse{}, apiErr, nil

	default:
		return -1, ctypes.APISuccessResponse{}, ctypes.APIErrorResponse{}, fmt.Errorf("unknown response status: %q", env.Status)
	}
}

func CallHttpAction[T any](path string, data map[string]any, requiresAuthorization bool, nodeToken string, httpMethod string) (T, error) {

	var result T

	baseURL := strings.TrimRight(os.Getenv("CONVEX_SITE_URL"), "/")
	if baseURL == "" {
		return result, fmt.Errorf("CONVEX_SITE_URL is empty")
	}
	url := baseURL + path

	var body []byte

	if data != nil {
		var err error
		body, err = json.Marshal(data)
		if err != nil {
			return result, fmt.Errorf("marshal body: %w", err)
		}
	}

	req, err := http.NewRequest(httpMethod, url, bytes.NewReader(body))
	if err != nil {
		return result, fmt.Errorf("create request: %w", err)
	}
	if requiresAuthorization {
		req.Header.Set("Authorization", "Bearer "+nodeToken)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	res, err := (&http.Client{}).Do(req)
	if err != nil {
		return result, fmt.Errorf("execute request: %w", err)
	}
	defer res.Body.Close()

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return result, fmt.Errorf("read response body: %w", err)
	}

	if res.StatusCode >= 400 {
		return result, fmt.Errorf("http %d: %s", res.StatusCode, string(raw))
	}

	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &result); err != nil {
			return result, fmt.Errorf("decode response: %w", err)
		}
	}

	return result, nil
}
