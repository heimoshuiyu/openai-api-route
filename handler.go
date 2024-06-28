package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type OpenAIAPI struct {
	Config *Config
	DB     *gorm.DB
}

func (o *OpenAIAPI) V1Handler(c *gin.Context) {
	hostname, _ := os.Hostname()
	if config.Hostname != "" {
		hostname = config.Hostname
	}
	record := Record{
		IP:            c.ClientIP(),
		Hostname:      hostname,
		CreatedAt:     time.Now(),
		Authorization: c.Request.Header.Get("Authorization"),
		UserAgent:     c.Request.Header.Get("User-Agent"),
		Model:         c.Request.URL.Path,
	}

	authorization := c.Request.Header.Get("Authorization")
	if strings.HasPrefix(authorization, "Bearer") {
		authorization = strings.Trim(authorization[len("Bearer"):], " ")
	} else {
		authorization = strings.Trim(authorization, " ")
		log.Println("[auth] Warning: authorization header should start with 'Bearer'")
	}
	log.Println("Received authorization '" + authorization + "'")

	// build avaliableUpstreams
	avaliableUpstreams := make([]OPENAI_UPSTREAM, 0)
	for _, upstream := range config.Upstreams {
		// noauth mode from cli arguments
		if !o.Config.CliConfig.Noauth || upstream.Noauth {
			avaliableUpstreams = append(avaliableUpstreams, upstream)
			continue
		}
		// check authorization header
		if checkAuth(authorization, upstream.Authorization) == nil {
			avaliableUpstreams = append(avaliableUpstreams, upstream)
			continue
		}
	}
	if len(avaliableUpstreams) == 0 {
		c.Header("Content-Type", "application/json")
		sendCORSHeaders(c)
		c.AbortWithError(403, fmt.Errorf("[processRequest.begin]: no avaliable upstream"))
		return
	} else if len(avaliableUpstreams) == 1 {
		avaliableUpstreams[0].Timeout = 120
	}

	for index, upstream := range avaliableUpstreams {
		var err error
		if upstream.SK == "" {
			sendCORSHeaders(c)
			c.AbortWithError(500, fmt.Errorf("[processRequest.begin]: invaild SK (secret key) '%s'", upstream.SK))
			continue
		}

		shouldResponse := index == len(avaliableUpstreams)-1

		if upstream.Type == "replicate" {
			err = processReplicateRequest(c, &upstream, &record, shouldResponse)
		} else if upstream.Type == "openai" {
			err = processRequest(c, &upstream, &record, shouldResponse)
		} else {
			err = fmt.Errorf("[processRequest.begin]: unsupported upstream type '%s'", upstream.Type)
		}

		if err != nil {
			if err == http.ErrAbortHandler {
				abortErr := "[processRequest.done]: AbortHandler, client's connection lost?, no upstream will try, stop here"
				log.Println(abortErr)
				record.Response += abortErr
				record.Status = 500
				break
			}
			log.Println("[processRequest.done]: Error from upstream", upstream.Endpoint, "should retry", err)
			continue
		}

		break
	}

	log.Println("[final]: Record result:", record.Status, record.Response)
	record.ElapsedTime = time.Since(record.CreatedAt)

	// async record request
	go func() {
		// encoder headers to record.Headers in json string
		headers, _ := json.Marshal(c.Request.Header)
		record.Headers = string(headers)

		// turncate request if too long
		log.Println("[async.record]: body length:", len(record.Body))
		if o.DB.Create(&record).Error != nil {
			log.Println("[async.record]: Error to save record:", record)
		}
	}()

	if record.Status != 200 {
		errMessage := fmt.Sprintf("[result.error]: IP: %s request %s error %d with %s", record.IP, record.Model, record.Status, record.Response)
		go sendFeishuMessage(errMessage)
		go sendMatrixMessage(errMessage)
	}
}
