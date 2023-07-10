package main

import (
	"encoding/json"
	"log"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type UserMessage struct {
	gorm.Model
	ModelName string
	Content   string
}

// sturcture to parse request
type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}
type Message struct {
	Content string `json:"content"`
}

func recordUserMessage(c *gin.Context, db *gorm.DB, body []byte) {
	bodyJson := ChatRequest{}
	err := json.Unmarshal(body, &bodyJson)
	if err != nil {
		c.AbortWithError(502, err)
		return
	}
	model := bodyJson.Model
	if !strings.HasPrefix(model, "gpt-") {
		return
	}
	// get message content
	if len(bodyJson.Messages) == 0 {
		return
	}
	content := bodyJson.Messages[len(bodyJson.Messages)-1].Content

	log.Println("Record user message", model, content)

	userMessage := UserMessage{
		ModelName: model,
		Content:   content,
	}
	db.Create(&userMessage)
}
