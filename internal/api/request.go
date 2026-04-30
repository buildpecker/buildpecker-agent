package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

// -1 - other failure (connection reset, transport failure)
// 0 - http failure
// 1 - success

var dbUrl = os.Getenv("CONVEX_PUBLIC_URL")

func Post(path string, contentType string, data map[string]any) (int, APISucessResponse, APIErrorResponse) {

	dbUrl := os.Getenv("CONVEX_PUBLIC_URL")
	url := dbUrl + "/" + path

	body, err := json.Marshal(&data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error while marshalling json for POST: '%s'\n", err)
		return -1, APISucessResponse{}, APIErrorResponse{}
	}

	res, err := http.Post(url, contentType, bytes.NewBuffer(body))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error in POST request: '%s'\n", err)
		return -1, APISucessResponse{}, APIErrorResponse{}
	}
	defer res.Body.Close()

	//http error
	if res.StatusCode >= 400 {
		apiData, err := CreateErrorResponse(res, data)
		if err != nil {
			return -1, APISucessResponse{}, APIErrorResponse{}
		}
		return 0, APISucessResponse{}, apiData
	}

	//http success
	apiData, err := CreateSuccessResponse(res, data)
	if err != nil {
		return -1, APISucessResponse{}, APIErrorResponse{}
	}
	return 1, apiData, APIErrorResponse{}
}
