package main

import (
	"encoding/json"
)

type RequestBody struct {
	Model  string `json:"model"`
	Stream bool   `json:"stream"`
}

func ParseRequestBody(data []byte) (RequestBody, error) {
	ret := RequestBody{
		Stream: false,
	}

	var requestBody RequestBody
	err := json.Unmarshal(data, &requestBody)
	if err != nil {
		return ret, err
	}

	return requestBody, nil
}
