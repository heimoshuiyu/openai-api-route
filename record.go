package main

import (
	"time"
)

type Record struct {
	ID               int64 `gorm:"primaryKey,autoIncrement"`
	Hostname         string
	UpstreamEndpoint string
	UpstreamSK       string
	CreatedAt        time.Time
	IP               string
	Body             string
	Model            string
	Response         string
	ResponseTime     time.Duration
	ElapsedTime      time.Duration
	Status           int
	Authorization    string // the autorization header send by client
	UserAgent        string
	Headers          string
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
