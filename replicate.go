package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

var replicate_model_url_template = "https://api.replicate.com/v1/models/%s/predictions"

func processReplicateRequest(c *gin.Context, upstream *OPENAI_UPSTREAM, record *Record, shouldResponse bool) error {
	err := _processReplicateRequest(c, upstream, record, shouldResponse)
	if shouldResponse {
		sendCORSHeaders(c)
		if err != nil {
			c.AbortWithError(502, err)
		}
	}
	return err
}

func _processReplicateRequest(c *gin.Context, upstream *OPENAI_UPSTREAM, record *Record, shouldResponse bool) error {
	// read request body
	inBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return errors.New("[processReplicateRequest]: failed to read request body " + err.Error())
	}

	// record request body
	record.Body = string(inBody)

	// parse request body
	inRequest := &OpenAIChatRequest{}
	err = json.Unmarshal(inBody, inRequest)
	if err != nil {
		c.Request.Body = io.NopCloser(bytes.NewBuffer(inBody))
		return errors.New("[processReplicateRequest]: failed to parse request body " + err.Error())
	}

	record.Model = inRequest.Model

	// check allow model
	if len(upstream.Allow) > 0 {
		isAllow := false
		for _, model := range upstream.Allow {
			if model == inRequest.Model {
				isAllow = true
				break
			}
		}
		if !isAllow {
			c.Request.Body = io.NopCloser(bytes.NewBuffer(inBody))
			return errors.New("[processReplicateRequest]: model not allow")
		}
	}
	// check block model
	if len(upstream.Deny) > 0 {
		for _, model := range upstream.Deny {
			if model == inRequest.Model {
				c.Request.Body = io.NopCloser(bytes.NewBuffer(inBody))
				return errors.New("[processReplicateRequest]: model deny")
			}
		}
	}

	// set url
	model_url := fmt.Sprintf(replicate_model_url_template, inRequest.Model)
	log.Println("[processReplicateRequest]: model_url:", model_url)

	// create request with default value
	outRequest := &ReplicateModelRequest{
		Stream: false,
		Input: ReplicateModelRequestInput{
			Prompt:           "",
			MaxNewTokens:     1024,
			Temperature:      0.6,
			Top_p:            0.9,
			Top_k:            50,
			FrequencyPenalty: 0.0,
			PresencePenalty:  0.0,
			PromptTemplate:   "{prompt}",
		},
	}

	// copy value from input request
	outRequest.Stream = inRequest.Stream
	outRequest.Input.Temperature = inRequest.Temperature
	outRequest.Input.FrequencyPenalty = inRequest.FrequencyPenalty
	outRequest.Input.PresencePenalty = inRequest.PresencePenalty

	// render prompt
	systemMessage := ""
	userMessage := ""
	assistantMessage := ""
	for _, message := range inRequest.Messages {
		if message.Role == "system" {
			if systemMessage != "" {
				systemMessage += "\n"
			}
			systemMessage += message.Content
			continue
		}
		if message.Role == "user" {
			if userMessage != "" {
				userMessage += "\n"
			}
			userMessage += message.Content
			if systemMessage != "" {
				userMessage = systemMessage + "\n" + userMessage
				systemMessage = ""
			}
			continue
		}
		if message.Role == "assistant" {
			if assistantMessage != "" {
				assistantMessage += "\n"
			}
			assistantMessage += message.Content

			if outRequest.Input.Prompt != "" {
				outRequest.Input.Prompt += "\n"
			}
			if userMessage != "" {
				outRequest.Input.Prompt += fmt.Sprintf("<s> [INST] %s [/INST] %s </s>", userMessage, assistantMessage)
			} else {
				outRequest.Input.Prompt += fmt.Sprintf("<s> %s </s>", assistantMessage)
			}
			userMessage = ""
			assistantMessage = ""
		}
		// unknown role
		log.Println("[processReplicateRequest]: Warning: unknown role", message.Role)
	}
	// final user message
	if userMessage != "" {
		outRequest.Input.Prompt += fmt.Sprintf("<s> [INST] %s [/INST] ", userMessage)
		userMessage = ""
	}
	// final assistant message
	if assistantMessage != "" {
		outRequest.Input.Prompt += fmt.Sprintf("<s> %s </s>", assistantMessage)
	}
	log.Println("[processReplicateRequest]: outRequest.Input.Prompt:", outRequest.Input.Prompt)

	// send request
	outBody, err := json.Marshal(outRequest)
	if err != nil {
		c.Request.Body = io.NopCloser(bytes.NewBuffer(inBody))
		return errors.New("[processReplicateRequest]: failed to marshal request body " + err.Error())
	}

	// http add headers
	req, err := http.NewRequest("POST", model_url, bytes.NewBuffer(outBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Token "+upstream.SK)
	// send
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.Request.Body = io.NopCloser(bytes.NewBuffer(inBody))
		return errors.New("[processReplicateRequest]: failed to post request " + err.Error())
	}

	// read response body
	outBody, err = io.ReadAll(resp.Body)
	if err != nil {
		c.Request.Body = io.NopCloser(bytes.NewBuffer(inBody))
		return errors.New("[processReplicateRequest]: failed to read response body " + err.Error())
	}

	// parse reponse body
	outResponse := &ReplicateModelResponse{}
	err = json.Unmarshal(outBody, outResponse)
	if err != nil {
		c.Request.Body = io.NopCloser(bytes.NewBuffer(inBody))
		return errors.New("[processReplicateRequest]: failed to parse response body " + err.Error())
	}

	if outResponse.Stream {
		// get result
		log.Println("[processReplicateRequest]: outResponse.URLS.Get:", outResponse.URLS.Stream)
		req, err := http.NewRequest("GET", outResponse.URLS.Stream, nil)
		if err != nil {
			c.Request.Body = io.NopCloser(bytes.NewBuffer(inBody))
			return errors.New("[processReplicateRequest]: failed to create get request " + err.Error())
		}
		req.Header.Set("Authorization", "Token "+upstream.SK)
		req.Header.Set("Accept", "text/event-stream")
		// send
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			c.Request.Body = io.NopCloser(bytes.NewBuffer(inBody))
			return errors.New("[processReplicateRequest]: failed to get request " + err.Error())
		}

		// get result by chunk
		var buffer string = ""
		var indexCount int64 = 0
		for {
			if !strings.Contains(buffer, "\n\n") {
				// receive chunk
				chunk := make([]byte, 1024)
				length, err := resp.Body.Read(chunk)
				if err == io.EOF {
					break
				}
				if length == 0 {
					break
				}
				if err != nil {
					c.Request.Body = io.NopCloser(bytes.NewBuffer(inBody))
					return errors.New("[processReplicateRequest]: failed to read response body " + err.Error())
				}
				// add chunk to buffer
				chunk = bytes.Trim(chunk, "\x00")
				buffer += string(chunk)
				continue
			}

			// cut the first chunk by find index
			index := strings.Index(buffer, "\n\n")
			chunk := buffer[:index]
			buffer = buffer[index+2:]

			// trim line
			chunk = strings.Trim(chunk, "\n")

			// ignore hi
			if !strings.Contains(chunk, "\n") {
				continue
			}

			// parse chunk to ReplicateModelResultChunk object
			chunkObj := &ReplicateModelResultChunk{}
			lines := strings.Split(chunk, "\n")
			// first line is event
			chunkObj.Event = strings.TrimSpace(lines[0])
			chunkObj.Event = strings.TrimPrefix(chunkObj.Event, "event: ")
			// second line is id
			chunkObj.ID = strings.TrimSpace(lines[1])
			chunkObj.ID = strings.TrimPrefix(chunkObj.ID, "id: ")
			chunkObj.ID = strings.SplitN(chunkObj.ID, ":", 2)[0]
			// third line is data
			chunkObj.Data = lines[2]
			chunkObj.Data = strings.TrimPrefix(chunkObj.Data, "data: ")

			record.Response += chunkObj.Data

			// done
			if chunkObj.Event == "done" {
				break
			}

			sendCORSHeaders(c)

			// create OpenAI response chunk
			c.SSEvent("", &OpenAIChatResponseChunk{
				ID:    "",
				Model: outResponse.Model,
				Choices: []OpenAIChatResponseChunkChoice{
					{
						Index: indexCount,
						Delta: OpenAIChatMessage{
							Role:    "assistant",
							Content: chunkObj.Data,
						},
					},
				},
			})
			c.Writer.Flush()
			indexCount += 1
		}
		sendCORSHeaders(c)
		c.SSEvent("", &OpenAIChatResponseChunk{
			ID:    "",
			Model: outResponse.Model,
			Choices: []OpenAIChatResponseChunkChoice{
				{
					Index: indexCount,
					Delta: OpenAIChatMessage{
						Role:    "assistant",
						Content: "",
					},
					FinishReason: "stop",
				},
			},
		})
		c.Writer.Flush()
		indexCount += 1
		record.Status = 200
		return nil

	} else {
		var result *ReplicateModelResultGet

		for {
			// get result
			log.Println("[processReplicateRequest]: outResponse.URLS.Get:", outResponse.URLS.Get)
			req, err := http.NewRequest("GET", outResponse.URLS.Get, nil)
			if err != nil {
				c.Request.Body = io.NopCloser(bytes.NewBuffer(inBody))
				return errors.New("[processReplicateRequest]: failed to create get request " + err.Error())
			}
			req.Header.Set("Authorization", "Token "+upstream.SK)
			// send
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				c.Request.Body = io.NopCloser(bytes.NewBuffer(inBody))
				return errors.New("[processReplicateRequest]: failed to get request " + err.Error())
			}
			// get result
			resultBody, err := io.ReadAll(resp.Body)
			if err != nil {
				c.Request.Body = io.NopCloser(bytes.NewBuffer(inBody))
				return errors.New("[processReplicateRequest]: failed to read response body " + err.Error())
			}

			// parse reponse body
			result = &ReplicateModelResultGet{}
			err = json.Unmarshal(resultBody, result)
			if err != nil {
				c.Request.Body = io.NopCloser(bytes.NewBuffer(inBody))
				return errors.New("[processReplicateRequest]: failed to parse response body " + err.Error())
			}

			if result.Status == "processing" || result.Status == "starting" {
				time.Sleep(3 * time.Second)
				continue
			}

			break
		}

		// build openai resposne
		openAIResult := &OpenAIChatResponse{
			ID:      result.ID,
			Model:   result.Model,
			Choices: []OpenAIChatResponseChoice{},
			Usage: OpenAIChatResponseUsage{
				TotalTokens:  result.Metrics.InputTokenCount + result.Metrics.OutputTokenCount,
				PromptTokens: result.Metrics.InputTokenCount,
			},
		}
		openAIResult.Choices = append(openAIResult.Choices, OpenAIChatResponseChoice{
			Index: 0,
			Message: OpenAIChatMessage{
				Role:    "assistant",
				Content: strings.Join(result.Output, ""),
			},
			FinishReason: "stop",
		})

		record.Body = strings.Join(result.Output, "")
		record.Status = 200

		// gin return
		sendCORSHeaders(c)
		c.JSON(200, openAIResult)

	}

	return nil
}
