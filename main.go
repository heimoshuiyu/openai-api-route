package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func main() {
	dbAddr := flag.String("database", "./db.sqlite", "Database address")
	listenAddr := flag.String("addr", ":8888", "Listening address")
	addMode := flag.Bool("add", false, "Add an OpenAI upstream")
	listMode := flag.Bool("list", false, "List all upstream")
	sk := flag.String("sk", "", "OpenAI API key (sk-xxxxx)")
	noauth := flag.Bool("noauth", false, "Do not check incoming authorization header")
	endpoint := flag.String("endpoint", "https://api.openai.com/v1", "OpenAI API base")
	flag.Parse()

	log.Println("Service starting")

	// connect to database
	db, err := gorm.Open(sqlite.Open(*dbAddr), &gorm.Config{
		PrepareStmt:            true,
		SkipDefaultTransaction: true,
	})
	if err != nil {
		log.Fatal("Failed to connect to database")
	}

	err = initconfig(db)
	if err != nil {
		log.Fatal(err)
	}

	db.AutoMigrate(&OPENAI_UPSTREAM{})
	db.AutoMigrate(&Record{})
	log.Println("Auto migrate database done")

	if *addMode {
		if *sk == "" {
			log.Fatal("Missing --sk flag")
		}
		newUpstream := OPENAI_UPSTREAM{}
		newUpstream.SK = *sk
		newUpstream.Endpoint = *endpoint
		err = db.Create(&newUpstream).Error
		if err != nil {
			log.Fatal("Can not add upstream", err)
		}
		log.Println("Successuflly add upstream", *sk, *endpoint)
		return
	}

	if *listMode {
		result := make([]OPENAI_UPSTREAM, 0)
		db.Find(&result)
		fmt.Println("SK\tEndpoint")
		for _, upstream := range result {
			fmt.Println(upstream.SK, upstream.Endpoint)
		}
		return
	}

	// init gin
	engine := gin.Default()

	// error handle middleware
	engine.Use(func(c *gin.Context) {
		c.Next()
		if len(c.Errors) == 0 {
			return
		}
		errText := strings.Join(c.Errors.Errors(), "\n")
		c.JSON(-1, gin.H{
			"error": errText,
		})
	})

	// CORS handler
	engine.OPTIONS("/v1/*any", func(ctx *gin.Context) {
		header := ctx.Writer.Header()
		header.Set("Access-Control-Allow-Origin", "*")
		header.Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, PATCH")
		header.Set("Access-Control-Allow-Headers", "Origin, Authorization, Content-Type")
		ctx.AbortWithStatus(200)
	})

	// get authorization config from db
	db.Take(&authConfig, "key = ?", "authorization")

	engine.POST("/v1/*any", func(c *gin.Context) {
		record := Record{
			IP:            c.ClientIP(),
			CreatedAt:     time.Now(),
			Authorization: c.Request.Header.Get("Authorization"),
		}
		defer func() {
			if err := recover(); err != nil {
				log.Println("Error:", err)
				c.AbortWithError(500, fmt.Errorf("%s", err))
			}
		}()

		// check authorization header
		if !*noauth {
			if handleAuth(c) != nil {
				return
			}
		}

		// get load balance policy
		policy := ConfigKV{Value: "main"}
		db.Take(&policy, "key = ?", "policy")
		log.Println("policy is", policy.Value)

		upstream := OPENAI_UPSTREAM{}

		// choose openai upstream
		switch policy.Value {
		case "main":
			db.Order("failed_count, success_count desc").First(&upstream)
		case "random":
			// randomly select one upstream
			db.Order("random()").Take(&upstream)
		case "random_available":
			// randomly select one non-failed upstream
			db.Where("failed_count = ?", 0).Order("random()").Take(&upstream)
		case "round_robin":
			// iterates each upstream
			db.Order("last_call_success_time").First(&upstream)
		case "round_robin_available":
			db.Where("failed_count = ?", 0).Order("last_call_success_time").First(&upstream)
		default:
			c.AbortWithError(500, fmt.Errorf("unknown load balance policy '%s'", policy.Value))
		}

		// do check
		log.Println("upstream is", upstream.SK, upstream.Endpoint)
		if upstream.Endpoint == "" || upstream.SK == "" {
			c.AbortWithError(500, fmt.Errorf("invaild upstream from '%s' policy", policy.Value))
			return
		}

		record.UpstreamID = upstream.ID

		// reverse proxy
		remote, err := url.Parse(upstream.Endpoint)
		if err != nil {
			c.AbortWithError(500, errors.New("can't parse reverse proxy remote URL"))
			return
		}
		proxy := httputil.NewSingleHostReverseProxy(remote)
		proxy.Director = nil
		proxy.Rewrite = func(proxyRequest *httputil.ProxyRequest) {
			in := proxyRequest.In
			out := proxyRequest.Out

			// read request body
			body, err := io.ReadAll(in.Body)
			if err != nil {
				c.AbortWithError(502, errors.New("reverse proxy middleware failed to read request body "+err.Error()))
				return
			}

			// record chat message from user
			record.Body = string(body)

			out.Body = io.NopCloser(bytes.NewReader(body))

			out.Host = remote.Host
			out.URL.Scheme = remote.Scheme
			out.URL.Host = remote.Host
			out.URL.Path = in.URL.Path
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
			record.Status = r.StatusCode
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
				record.Response = fmt.Sprintf("upstream return '%s' with '%s'", r.Status, string(body))
				return fmt.Errorf(record.Response)
			}
			// count success
			r.Body = io.NopCloser(io.TeeReader(r.Body, &buf))
			contentType = r.Header.Get("content-type")
			return nil
		}
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			log.Println("Error", err, upstream.SK, upstream.Endpoint)

			// abort to error handle
			c.AbortWithError(502, err)

			// send notification
			upstreams := []OPENAI_UPSTREAM{}
			db.Find(&upstreams)
			content := fmt.Sprintf("[%s] OpenAI 转发出错 ID: %d... 密钥: [%s] 上游: [%s] 错误: %s",
				c.ClientIP(),
				upstream.ID, upstream.SK, upstream.Endpoint, err.Error(),
			)
			if err.Error() != "context canceled" && r.Response.StatusCode != 400 {
				go sendMatrixMessage(content)
				go sendFeishuMessage(content)
			}

			log.Println("response is", r.Response)
		}

		func() {
			defer func() {
				if err := recover(); err != nil {
					log.Println("Panic recover :", err)
				}
			}()
			proxy.ServeHTTP(c.Writer, c.Request)
		}()

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
				record.Response = fetchResp.Choices[0].Message.Content
			} else {
				log.Println("Unknown content type", contentType)
				return
			}
		}

		if len(record.Body) > 1024*512 {
			record.Body = ""
		}

		log.Println("Record result:", record.Response)
		record.ElapsedTime = time.Now().Sub(record.CreatedAt)
		if db.Create(&record).Error != nil {
			log.Println("Error to save record:", record)
		}
	})

	engine.Run(*listenAddr)
}
