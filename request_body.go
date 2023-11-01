package main

import (
	"encoding/json"
)

type Message struct {
	Content string `json:"content"`
	Role    string `json:"role"`
}

type RequestBody struct {
	Model            string    `json:"model"`
	Messages         []Message `json:"messages"`
	Stream           bool      `json:"stream"`
	Temperature      float64   `json:"temperature"`
	TopP             int64     `json:"top_p"`
	PresencePenalty  float64   `json:"presence_penalty"`
	FrequencyPenalty float64   `json:"frequency_penalty"`
}

func ParseRequestBody(data []byte) (RequestBody, error) {
	ret := RequestBody{}

	var requestBody RequestBody
	err := json.Unmarshal(data, &requestBody)
	if err != nil {
		return ret, err
	}

	return requestBody, nil
}
