package main

import (
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Record struct {
	ID               int64 `gorm:"primaryKey,autoIncrement"`
	UpstreamEndpoint string
	CreatedAt        time.Time
	IP               string
	Body             string `gorm:"serializer:json"`
	Model            string
	Response         string
	ResponseTime     time.Duration
	ElapsedTime      time.Duration
	Status           int
	UpstreamID       uint
	Authorization    string
}

type StreamModeChunk struct {
	Choices []StreamModeChunkChoice `json:"choices"`
}
type StreamModeChunkChoice struct {
	Delta        StreamModeDelta `json:"delta"`
	FinishReason string          `json:"finish_reason"`
}
type StreamModeDelta struct {
	Content string `json:"content"`
}

type FetchModeResponse struct {
	Model   string            `json:"model"`
	Choices []FetchModeChoice `json:"choices"`
	Usage   FetchModeUsage    `json:"usage"`
}
type FetchModeChoice struct {
	Message      FetchModeMessage `json:"message"`
	FinishReason string           `json:"finish_reason"`
}
type FetchModeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type FetchModeUsage struct {
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
}

func recordAssistantResponse(contentType string, db *gorm.DB, trackID uuid.UUID, body []byte, elapsedTime time.Duration) {
	result := ""
	// stream mode
	if strings.HasPrefix(contentType, "text/event-stream") {
		resp := string(body)
		for _, line := range strings.Split(resp, "\n") {
			chunk := StreamModeChunk{}
			line = strings.TrimPrefix(line, "data:")
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			err := json.Unmarshal([]byte(line), &chunk)
			if err != nil {
				log.Println(err)
				continue
			}

			if len(chunk.Choices) == 0 {
				continue
			}
			result += chunk.Choices[0].Delta.Content
		}
	} else if strings.HasPrefix(contentType, "application/json") {
		var fetchResp FetchModeResponse
		err := json.Unmarshal(body, &fetchResp)
		if err != nil {
			log.Println("Error parsing fetch response:", err)
			return
		}
		if !strings.HasPrefix(fetchResp.Model, "gpt-") {
			log.Println("Not GPT model, skip recording response:", fetchResp.Model)
			return
		}
		if len(fetchResp.Choices) == 0 {
			log.Println("Error: fetch response choice length is 0")
			return
		}
		result = fetchResp.Choices[0].Message.Content
	} else {
		log.Println("Unknown content type", contentType)
		return
	}
	log.Println("Record result:", result)
	record := Record{}
	if db.Find(&record, "id = ?", trackID).Error != nil {
		log.Println("Error find request record with trackID:", trackID)
		return
	}
	record.Response = result
	record.ElapsedTime = elapsedTime
	if db.Save(&record).Error != nil {
		log.Println("Error to save record:", record)
		return
	}
}
