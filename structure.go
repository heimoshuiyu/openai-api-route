package main

import (
	"net/url"
)

type OPENAI_UPSTREAM struct {
	SK            string   `yaml:"sk"`
	Endpoint      string   `yaml:"endpoint"`
	Timeout       int64    `yaml:"timeout"`
	StreamTimeout int64    `yaml:"stream_timeout"`
	Allow         []string `yaml:"allow"`
	Deny          []string `yaml:"deny"`
	Type          string   `yaml:"type"`
	KeepHeader    bool     `yaml:"keep_header"`
	Authorization string   `yaml:"authorization"`
	Noauth        bool     `yaml:"noauth"`
	URL           *url.URL
}

type OpenAIChatRequest struct {
	FrequencyPenalty float64 `json:"frequency_penalty"`
	PresencePenalty  float64 `json:"presence_penalty"`
	MaxTokens        int64   `json:"max_tokens"`
	Model            string  `json:"model"`
	Stream           bool    `json:"stream"`
	Temperature      float64 `json:"temperature"`
	Messages         []OpenAIChatRequestMessage
}

type OpenAIChatRequestMessage struct {
	Content string `json:"content"`
	Role    string `json:"role"`
}

type ReplicateModelRequest struct {
	Stream bool                       `json:"stream"`
	Input  ReplicateModelRequestInput `json:"input"`
}

type ReplicateModelRequestInput struct {
	Prompt           string  `json:"prompt"`
	MaxNewTokens     int64   `json:"max_new_tokens"`
	Temperature      float64 `json:"temperature"`
	Top_p            float64 `json:"top_p"`
	Top_k            int64   `json:"top_k"`
	PresencePenalty  float64 `json:"presence_penalty"`
	FrequencyPenalty float64 `json:"frequency_penalty"`
	PromptTemplate   string  `json:"prompt_template"`
}

type ReplicateModelResponse struct {
	Model   string                     `json:"model"`
	Version string                     `json:"version"`
	Stream  bool                       `json:"stream"`
	Error   string                     `json:"error"`
	URLS    ReplicateModelResponseURLS `json:"urls"`
}

type ReplicateModelResponseURLS struct {
	Cancel string `json:"cancel"`
	Get    string `json:"get"`
	Stream string `json:"stream"`
}

type ReplicateModelResultGet struct {
	ID      string                      `json:"id"`
	Model   string                      `json:"model"`
	Version string                      `json:"version"`
	Output  []string                    `json:"output"`
	Error   string                      `json:"error"`
	Metrics ReplicateModelResultMetrics `json:"metrics"`
	Status  string                      `json:"status"`
}

type ReplicateModelResultMetrics struct {
	InputTokenCount  int64 `json:"input_token_count"`
	OutputTokenCount int64 `json:"output_token_count"`
}

type OpenAIChatResponse struct {
	ID      string                     `json:"id"`
	Object  string                     `json:"object"`
	Created int64                      `json:"created"`
	Model   string                     `json:"model"`
	Choices []OpenAIChatResponseChoice `json:"choices"`
	Usage   OpenAIChatResponseUsage    `json:"usage"`
}

type OpenAIChatResponseUsage struct {
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
}

type OpenAIChatResponseChoice struct {
	Index        int64             `json:"index"`
	Message      OpenAIChatMessage `json:"message"`
	FinishReason string            `json:"finish_reason"`
}

type OpenAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ReplicateModelResultChunk struct {
	Event string `json:"event"`
	ID    string `json:"id"`
	Data  string `json:"data"`
}

type OpenAIChatResponseChunk struct {
	ID      string                          `json:"id"`
	Object  string                          `json:"object"`
	Created int64                           `json:"created"`
	Model   string                          `json:"model"`
	Choices []OpenAIChatResponseChunkChoice `json:"choices"`
}

type OpenAIChatResponseChunkChoice struct {
	Index        int64             `json:"index"`
	Delta        OpenAIChatMessage `json:"delta"`
	FinishReason string            `json:"finish_reason"`
}
