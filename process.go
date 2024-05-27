package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func processRequest(c *gin.Context, upstream *OPENAI_UPSTREAM, record *Record, shouldResponse bool) error {

	// reverse proxy
	remote, err := url.Parse(upstream.Endpoint)
	if err != nil {
		return err
	}

	path := strings.TrimPrefix(c.Request.URL.Path, "/v1")
	remote.Path = upstream.URL.Path + path
	log.Println("[proxy.begin]:", remote)
	log.Println("[proxy.begin]: shouldResposne:", shouldResponse)

	client := &http.Client{}
	request := &http.Request{}
	request.ContentLength = c.Request.ContentLength
	request.Method = c.Request.Method
	request.URL = remote

	// process header
	if upstream.KeepHeader {
		request.Header = c.Request.Header
	} else {
		request.Header = http.Header{}
	}

	// process header authorization
	if upstream.SK == "asis" {
		request.Header.Set("Authorization", c.Request.Header.Get("Authorization"))
	} else {
		request.Header.Set("Authorization", "Bearer "+upstream.SK)
	}
	request.Header.Set("Content-Type", c.Request.Header.Get("Content-Type"))
	request.Header.Set("Host", remote.Host)
	request.Header.Set("Content-Length", c.Request.Header.Get("Content-Length"))

	request.Body = c.Request.Body

	resp, err := client.Do(request)
	if err != nil {
		body := []byte{}
		if resp != nil && resp.Body != nil {
			body, _ = io.ReadAll(resp.Body)
		}
		return errors.New(err.Error() + " " + string(body))
	}

	defer resp.Body.Close()

	record.Status = resp.StatusCode

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		record.Status = resp.StatusCode
		errRet := fmt.Errorf("[error]: openai-api-route upstream return '%s' with '%s'", resp.Status, string(body))
		log.Println(errRet)
		return errRet
	}

	// copy response header
	for k, v := range resp.Header {
		c.Header(k, v[0])
	}
	sendCORSHeaders(c)

	respBodyBuffer := bytes.NewBuffer(make([]byte, 0, 4*1024))
	respBodyTeeReader := io.TeeReader(resp.Body, respBodyBuffer)
	record.ResponseTime = time.Since(record.CreatedAt)
	io.Copy(c.Writer, respBodyTeeReader)
	record.ElapsedTime = time.Since(record.CreatedAt)

	// parse and record response
	if strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") {
		var fetchResp FetchModeResponse
		err := json.NewDecoder(respBodyBuffer).Decode(&fetchResp)
		if err == nil {
			if len(fetchResp.Choices) > 0 {
				record.Response = fetchResp.Choices[0].Message.Content
			}
		}
	} else if strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		lines := bytes.Split(respBodyBuffer.Bytes(), []byte("\n"))
		for _, line := range lines {
			line = bytes.TrimSpace(line)
			line = bytes.TrimPrefix(line, []byte("data:"))
			line = bytes.TrimSpace(line)
			if len(line) == 0 {
				continue
			}
			chunk := StreamModeChunk{}
			err = json.Unmarshal(line, &chunk)
			if err != nil {
				log.Println("[proxy.parseChunkError]:", err)
				break
			}
			if len(chunk.Choices) == 0 {
				continue
			}
			record.Response += chunk.Choices[0].Delta.Content
		}
	} else if strings.HasPrefix(resp.Header.Get("Content-Type"), "text") {
		body, _ := io.ReadAll(respBodyBuffer)
		record.Response = string(body)
	} else {
		log.Println("[proxy.record]: Unknown content type", resp.Header.Get("Content-Type"))
	}

	return nil
}
