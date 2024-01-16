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
	var errCtx []error

	record.UpstreamEndpoint = upstream.Endpoint
	record.UpstreamSK = upstream.SK
	record.Response = ""
	// [TODO] record request body

	// reverse proxy
	remote, err := url.Parse(upstream.Endpoint)
	if err != nil {
		return err
	}

	remote.Path = upstream.URL.Path + strings.TrimPrefix(c.Request.URL.Path, "/v1")
	log.Println("[proxy.begin]:", remote)
	log.Println("[proxy.begin]: shouldResposne:", shouldResponse)

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
			errCtx = append(errCtx, errors.New("[proxy.rewrite]: reverse proxy middleware failed to read request body "+err.Error()))
			return
		}

		// record chat message from user
		record.Body = string(inBody)
		requestBody, requestBodyOK := ParseRequestBody(inBody)
		// record if parse success
		if requestBodyOK == nil {
			record.Model = requestBody.Model
			// check allow list
			if len(upstream.Allow) > 0 {
				isAllow := false
				for _, allow := range upstream.Allow {
					if allow == requestBody.Model {
						isAllow = true
						break
					}
				}
				if !isAllow {
					errCtx = append(errCtx, errors.New("[proxy.rewrite]: model not allowed"))
					return
				}
			}
			// check block list
			if len(upstream.Deny) > 0 {
				for _, deny := range upstream.Deny {
					if deny == requestBody.Model {
						errCtx = append(errCtx, errors.New("[proxy.rewrite]: model denied"))
						return
					}
				}
			}
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
				log.Println("[proxy.timeout]: Timeout upstream", upstream.Endpoint, timeout)
				errTimeout := errors.New("[proxy.timeout]: Timeout upstream")
				errCtx = append(errCtx, errTimeout)
				if shouldResponse {
					c.Header("Content-Type", "application/json")
					sendCORSHeaders(c)
					c.AbortWithError(502, errTimeout)
				}
				cancel()
			}
		}()

		out.Body = io.NopCloser(bytes.NewReader(inBody))

		out.Host = remote.Host
		out.URL.Scheme = remote.Scheme
		out.URL.Host = remote.Host

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

		// handle reverse proxy cors header if upstream do not set that
		if r.Header.Get("Access-Control-Allow-Origin") == "" {
			c.Header("Access-Control-Allow-Origin", "*")
		}
		if r.Header.Get("Access-Control-Allow-Methods") == "" {
			c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, PATCH")
		}
		if r.Header.Get("Access-Control-Allow-Headers") == "" {
			c.Header("Access-Control-Allow-Headers", "Origin, Authorization, Content-Type")
		}

		if !shouldResponse && r.StatusCode != 200 {
			log.Println("[proxy.modifyResponse]: upstream return not 200 and should not response", r.StatusCode)
			return errors.New("upstream return not 200 and should not response")
		}

		if r.StatusCode != 200 {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				errRet := errors.New("[proxy.modifyResponse]: failed to read response from upstream " + err.Error())
				return errRet
			}
			errRet := errors.New(fmt.Sprintf("[error]: openai-api-route upstream return '%s' with '%s'", r.Status, string(body)))
			log.Println(errRet)
			record.Status = r.StatusCode
			return errRet
		}
		// count success
		r.Body = io.NopCloser(io.TeeReader(r.Body, &buf))
		contentType = r.Header.Get("content-type")
		return nil
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		haveResponse = true
		record.ResponseTime = time.Now().Sub(record.CreatedAt)
		log.Println("[proxy.errorHandler]", err, upstream.SK, upstream.Endpoint)

		errCtx = append(errCtx, err)

		// abort to error handle
		if shouldResponse {
			c.Header("Content-Type", "application/json")
			sendCORSHeaders(c)
			for _, err := range errCtx {
				c.AbortWithError(502, err)
			}
		}

		log.Println("[proxy.errorHandler]: response is", r.Response)

		if record.Status == 0 {
			record.Status = 502
		}
		record.Response += "[proxy.ErrorHandler]: " + err.Error()
		if r.Response != nil {
			record.Status = r.Response.StatusCode
		}

	}

	err = ServeHTTP(proxy, c.Writer, c.Request)
	if err != nil {
		log.Println("[proxy.serve]: error from ServeHTTP:", err)
		// panic means client has abort the http connection
		// since the connection is lost, we return
		// and the reverse process should not try the next upsteam
		return http.ErrAbortHandler
	}

	// return context error
	if len(errCtx) > 0 {
		log.Println("[proxy.serve]: error from ServeHTTP:", errCtx)
		// fix inrequest body
		c.Request.Body = io.NopCloser(bytes.NewReader(inBody))
		return errCtx[len(errCtx)-1]
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
					log.Println("[proxy.parseChunkError]:", err)
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
				log.Println("[proxy.parseJSONError]: error parsing fetch response:", err)
				return nil
			}
			if !strings.HasPrefix(fetchResp.Model, "gpt-") {
				log.Println("[proxy.record]: Not GPT model, skip recording response:", fetchResp.Model)
				return nil
			}
			if len(fetchResp.Choices) == 0 {
				log.Println("[proxy.record]: Error: fetch response choice length is 0")
				return nil
			}
			record.Response = fetchResp.Choices[0].Message.Content
		} else {
			log.Println("[proxy.record]: Unknown content type", contentType)
		}
	}

	return nil
}
