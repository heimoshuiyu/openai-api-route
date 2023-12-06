package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/context"
)

func processRequest(c *gin.Context, upstream *OPENAI_UPSTREAM, record *Record, shouldResponse bool) error {
	var errCtx error

	record.UpstreamEndpoint = upstream.Endpoint
	record.UpstreamSK = upstream.SK
	record.Response = ""
	// [TODO] record request body

	// reverse proxy
	remote, err := url.Parse(upstream.Endpoint)
	if err != nil {
		c.AbortWithError(500, errors.New("can't parse reverse proxy remote URL"))
		return err
	}

	haveResponse := false

	proxy := httputil.NewSingleHostReverseProxy(remote)
	proxy.Director = nil
	var inBody []byte
	proxy.Rewrite = func(proxyRequest *httputil.ProxyRequest) {
		in := proxyRequest.In

		ctx, cancel := context.WithCancel(context.Background())
		proxyRequest.Out = proxyRequest.Out.WithContext(ctx)

		out := proxyRequest.Out

		// read request body
		inBody, err = io.ReadAll(in.Body)
		if err != nil {
			c.AbortWithError(502, errors.New("reverse proxy middleware failed to read request body "+err.Error()))
			return
		}

		// record chat message from user
		record.Body = string(inBody)
		requestBody, requestBodyOK := ParseRequestBody(inBody)
		// record if parse success
		if requestBodyOK == nil {
			record.Model = requestBody.Model
		}

		// set timeout, default is 60 second
		timeout := 60 * time.Second
		if requestBodyOK == nil && requestBody.Stream {
			timeout = 5 * time.Second
		}
		if len(inBody) > 1024*128 {
			timeout = 20 * time.Second
		}
		if upstream.Timeout > 0 {
			// convert upstream.Timeout(second) to nanosecond
			timeout = time.Duration(upstream.Timeout) * time.Second
		}

		// timeout out request
		go func() {
			time.Sleep(timeout)
			if !haveResponse {
				log.Println("Timeout upstream", upstream.Endpoint)
				errCtx = errors.New("timeout")
				if shouldResponse {
					c.AbortWithError(502, errCtx)
				}
				cancel()
			}
		}()

		out.Body = io.NopCloser(bytes.NewReader(inBody))

		out.Host = remote.Host
		out.URL.Scheme = remote.Scheme
		out.URL.Host = remote.Host

		out.URL.Path = upstream.URL.Path + strings.TrimPrefix(in.URL.Path, "/v1")

		out.Header = http.Header{}
		out.Header.Set("Host", remote.Host)
		if upstream.SK == "asis" {
			out.Header.Set("Authorization", c.Request.Header.Get("Authorization"))
		} else {
			out.Header.Set("Authorization", "Bearer "+upstream.SK)
		}
		out.Header.Set("Content-Type", c.Request.Header.Get("Content-Type"))
	}
	var buf bytes.Buffer
	var contentType string
	proxy.ModifyResponse = func(r *http.Response) error {
		haveResponse = true
		record.ResponseTime = time.Now().Sub(record.CreatedAt)
		record.Status = r.StatusCode
		if !shouldResponse && r.StatusCode != 200 {
			log.Println("upstream return not 200 and should not response", r.StatusCode)
			return errors.New("upstream return not 200 and should not response")
		}
		r.Header.Del("Access-Control-Allow-Origin")
		r.Header.Del("Access-Control-Allow-Methods")
		r.Header.Del("Access-Control-Allow-Headers")
		r.Header.Set("Access-Control-Allow-Origin", "*")
		r.Header.Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, PATCH")
		r.Header.Set("Access-Control-Allow-Headers", "Origin, Authorization, Content-Type")

		if r.StatusCode != 200 {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				record.Response = "failed to read response from upstream " + err.Error()
				return errors.New(record.Response)
			}
			record.Response = fmt.Sprintf("openai-api-route upstream return '%s' with '%s'", r.Status, string(body))
			record.Status = r.StatusCode
			return fmt.Errorf(record.Response)
		}
		// count success
		r.Body = io.NopCloser(io.TeeReader(r.Body, &buf))
		contentType = r.Header.Get("content-type")
		return nil
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		haveResponse = true
		record.ResponseTime = time.Now().Sub(record.CreatedAt)
		log.Println("Error", err, upstream.SK, upstream.Endpoint)

		errCtx = err

		// abort to error handle
		if shouldResponse {
			c.AbortWithError(502, err)
		}

		log.Println("response is", r.Response)

		if record.Status == 0 {
			record.Status = 502
		}
		if record.Response == "" {
			record.Response = err.Error()
		}
		if r.Response != nil {
			record.Status = r.Response.StatusCode
		}

	}

	proxy.ServeHTTP(c.Writer, c.Request)

	// return context error
	if errCtx != nil {
		// fix inrequest body
		c.Request.Body = io.NopCloser(bytes.NewReader(inBody))
		return errCtx
	}

	resp, err := io.ReadAll(io.NopCloser(&buf))
	if err != nil {
		record.Response = "failed to read response from upstream " + err.Error()
		log.Println(record.Response)
	} else {

		// record response
		// stream mode
		if strings.HasPrefix(contentType, "text/event-stream") {
			for _, line := range strings.Split(string(resp), "\n") {
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
				record.Response += chunk.Choices[0].Delta.Content
			}
		} else if strings.HasPrefix(contentType, "application/json") {
			var fetchResp FetchModeResponse
			err := json.Unmarshal(resp, &fetchResp)
			if err != nil {
				log.Println("Error parsing fetch response:", err)
				return nil
			}
			if !strings.HasPrefix(fetchResp.Model, "gpt-") {
				log.Println("Not GPT model, skip recording response:", fetchResp.Model)
				return nil
			}
			if len(fetchResp.Choices) == 0 {
				log.Println("Error: fetch response choice length is 0")
				return nil
			}
			record.Response = fetchResp.Choices[0].Message.Content
		} else {
			log.Println("Unknown content type", contentType)
		}
	}

	if len(record.Body) > 1024*512 {
		record.Body = ""
	}

	return nil
}
