package api

import "fmt"

func RegisterNode(authToken string) {
	data := map[string]any{
		"path": "users/queries:findOneByAuthToken",
		"args": map[string]any{
			"authToken": authToken,
		},
		"format": "json",
	}

	status, successData, errorData := Post("api/query", "application/json", data)

	switch status {
	case 0:
		fmt.Print(errorData)
	case 1:
		fmt.Print(successData)
	}
}
