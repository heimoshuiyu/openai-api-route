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

	path := strings.TrimPrefix(c.Request.URL.Path, "/v1")
	// recoognize whisper url
	remote.Path = upstream.URL.Path + path
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
			errCtx = append(errCtx, ErrReadRequestBody)
			return
		}

		// record chat message from user
		requestBody, requestBodyOK := ParseRequestBody(inBody)
		// record if parse success
		if requestBodyOK == nil && requestBody.Model != "" {
			record.Model = requestBody.Model
			record.Body = string(inBody)
		}

		// check allow list
		if len(upstream.Allow) > 0 {
			isAllow := false
			for _, allow := range upstream.Allow {
				if allow == record.Model {
					isAllow = true
					break
				}
			}
			if !isAllow {
				errCtx = append(errCtx, errors.New("[proxy.rewrite]: model '"+record.Model+"' not allowed"))
				return
			}
		}
		// check block list
		if len(upstream.Deny) > 0 {
			for _, deny := range upstream.Deny {
				if deny == record.Model {
					errCtx = append(errCtx, errors.New("[proxy.rewrite]: model '"+record.Model+"' denied"))
					return
				}
			}
		}

		// set timeout, default is 60 second
		timeout := time.Duration(upstream.Timeout) * time.Second
		if requestBodyOK == nil && requestBody.Stream {
			timeout = time.Duration(upstream.StreamTimeout) * time.Second
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

		if !upstream.KeepHeader {
			out.Header = http.Header{}
		}
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
		record.ResponseTime = time.Since(record.CreatedAt)
		record.Status = r.StatusCode

		// remove response's cors headers
		r.Header.Del("Access-Control-Allow-Origin")
		r.Header.Del("Access-Control-Allow-Methods")
		r.Header.Del("Access-Control-Allow-Headers")
		r.Header.Del("access-control-allow-origin")
		r.Header.Del("access-control-allow-methods")
		r.Header.Del("access-control-allow-headers")

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
			errRet := fmt.Errorf("[error]: openai-api-route upstream return '%s' with '%s'", r.Status, string(body))
			log.Println(errRet)
			record.Status = r.StatusCode
			return errRet
		}
		// handle reverse proxy cors header if upstream do not set that
		sendCORSHeaders(c)
		// count success
		r.Body = io.NopCloser(io.TeeReader(r.Body, &buf))
		contentType = r.Header.Get("content-type")
		return nil
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		haveResponse = true
		record.ResponseTime = time.Since(record.CreatedAt)
		log.Println("[proxy.errorHandler]", err, upstream.SK, upstream.Endpoint, errCtx)

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
		} else if strings.HasPrefix(contentType, "text") {
			record.Response = string(resp)
		} else if strings.HasPrefix(contentType, "application/json") {
			// fallback record response
			if len(resp) < 1024*128 {
				record.Response = string(resp)
			}
			var fetchResp FetchModeResponse
			err := json.Unmarshal(resp, &fetchResp)
			if err == nil {
				if len(fetchResp.Choices) > 0 {
					record.Response = fetchResp.Choices[0].Message.Content
				}
			}
		} else {
			log.Println("[proxy.record]: Unknown content type", contentType)
		}
	}

	return nil
}
