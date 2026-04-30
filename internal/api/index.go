package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

type APISucessResponse struct {
	Status   string
	Value    map[string]any
	LogLines []string
}

type APIErrorResponse struct {
	Status       string
	ErrorMessage string
	ErrorData    map[string]any
	LogLines     []string
}

func CreateSuccessResponse(res *http.Response, data map[string]any) (APISucessResponse, error) {
	var apiData APISucessResponse
	if err := json.NewDecoder(res.Body).Decode(&apiData); err != nil {
		fmt.Fprintf(os.Stderr, "Error in JSON parsing: '%s'\n", err)
		return APISucessResponse{}, err
	}
	return apiData, nil
}

func CreateErrorResponse(res *http.Response, data map[string]any) (APIErrorResponse, error) {
	var apiData APIErrorResponse
	if err := json.NewDecoder(res.Body).Decode(&apiData); err != nil {
		fmt.Fprintf(os.Stderr, "Error in JSON parsing: '%s'\n", err)
		return APIErrorResponse{}, err
	}
	return apiData, nil
}
