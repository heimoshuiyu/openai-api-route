package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"
)

type FeishuMessage struct {
	MsgType string               `json:"msg_type"`
	Content FeishuMessageContent `json:"content"`
}
type FeishuMessageContent struct {
	Text string `json:"text"`
}

func sendFeishuMessage(content string) error {
	messageBytes, err := json.Marshal(&FeishuMessage{
		MsgType: "text",
		Content: FeishuMessageContent{
			Text: content,
		},
	})
	if err != nil {
		log.Println("Failed to send feishu message", err)
	}
	FEISHU_WEBHOOK := os.Getenv("FEISHU_WEBOOK")
	if FEISHU_WEBHOOK == "" {
		log.Println("FEISHU_WEBOOK environment not set")
		return nil
	}
	http.Post(
		FEISHU_WEBHOOK,
		"application/json",
		bytes.NewReader(messageBytes),
	)
	return nil
}
