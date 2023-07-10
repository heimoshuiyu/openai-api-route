package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"
)

type MatrixMessage struct {
	Message string `json:"message"`
	Body    string `json:"body"`
}

func sendMatrixMessage(content string) error {
	messageBytes, marshalErr := json.Marshal(&MatrixMessage{
		Message: "m.text",
		Body:    content,
	})
	if marshalErr != nil {
		log.Println("Failed to send matrix message", marshalErr)
		return marshalErr
	}

	MATRIX_API := os.Getenv("MATRIX_API")
	if MATRIX_API == "" {
		log.Println("MATRIX_API envitonment not set")
		return nil
	}

	http.Post(
		MATRIX_API,
		"application/json",
		bytes.NewReader(messageBytes),
	)
	return nil
}
