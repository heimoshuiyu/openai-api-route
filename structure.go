package main

import (
	"log"
	"net/url"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Address       string            `yaml:"address"`
	Hostname      string            `yaml:"hostname"`
	DBType        string            `yaml:"dbtype"`
	DBAddr        string            `yaml:"dbaddr"`
	Authorization string            `yaml:"authorization"`
	Upstreams     []OPENAI_UPSTREAM `yaml:"upstreams"`
}
type OPENAI_UPSTREAM struct {
	SK       string   `yaml:"sk"`
	Endpoint string   `yaml:"endpoint"`
	Timeout  int64    `yaml:"timeout"`
	Allow    []string `yaml:"allow"`
	Deny     []string `yaml:"deny"`
	Type     string   `yaml:"type"`
	URL      *url.URL
}

func readConfig(filepath string) Config {
	var config Config

	// read yaml file
	data, err := os.ReadFile(filepath)
	if err != nil {
		log.Fatalf("Error reading YAML file: %s", err)
	}

	// Unmarshal the YAML into the upstreams slice
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		log.Fatalf("Error unmarshaling YAML: %s", err)
	}

	// set default value
	if config.Address == "" {
		log.Println("Address not set, use default value: :8888")
		config.Address = ":8888"
	}
	if config.DBType == "" {
		log.Println("DBType not set, use default value: sqlite")
		config.DBType = "sqlite"
	}
	if config.DBAddr == "" {
		log.Println("DBAddr not set, use default value: ./db.sqlite")
		config.DBAddr = "./db.sqlite"
	}

	for i, upstream := range config.Upstreams {
		// parse upstream endpoint URL
		endpoint, err := url.Parse(upstream.Endpoint)
		if err != nil {
			log.Fatalf("Can't parse upstream endpoint URL '%s': %s", upstream.Endpoint, err)
		}
		config.Upstreams[i].URL = endpoint
		if upstream.Type == "" {
			upstream.Type = "openai"
		}
		if (upstream.Type != "openai") && (upstream.Type != "replicate") {
			log.Fatalf("Unsupported upstream type '%s'", upstream.Type)
		}
	}

	return config
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
	User    string `json:"user"`
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
