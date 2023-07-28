package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
		fmt.Println("SK\tEndpoint\tSuccess\tFailed\tLast Success Time")
		for _, upstream := range result {
			fmt.Println(upstream.SK, upstream.Endpoint, upstream.SuccessCount, upstream.FailedCount, upstream.LastCallSuccessTime)
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
	engine.Use(handleCORS)

	// get authorization config from db
	db.Take(&authConfig, "key = ?", "authorization")

	engine.POST("/v1/*any", func(c *gin.Context) {
		trackID := uuid.New()
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
			go recordUserMessage(c, db, trackID, body)

			out.Body = io.NopCloser(bytes.NewReader(body))

			out.Host = remote.Host
			out.URL.Scheme = remote.Scheme
			out.URL.Host = remote.Host
			out.URL.Path = in.URL.Path
			out.Header = http.Header{}
			out.Header.Set("Host", remote.Host)
			out.Header.Set("Authorization", "Bearer "+upstream.SK)
			out.Header.Set("Content-Type", c.Request.Header.Get("Content-Type"))
		}
		var buf bytes.Buffer
		var contentType string
		proxy.ModifyResponse = func(r *http.Response) error {
			if r.StatusCode != 200 {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					return errors.New("failed to read response from upstream " + err.Error())
				}
				return fmt.Errorf("upstream return '%s' with '%s'", r.Status, string(body))
			}
			// count success
			go db.Model(&upstream).Updates(map[string]interface{}{
				"success_count":          gorm.Expr("success_count + ?", 1),
				"last_call_success_time": time.Now(),
			})
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
			upstreamDescriptions := make([]string, 0)
			for _, upstream := range upstreams {
				upstreamDescriptions = append(upstreamDescriptions, fmt.Sprintf("ID: %d, %s: %s 成功次数: %d, 失败次数: %d, 最后成功调用: %s",
					upstream.ID, upstream.SK, upstream.Endpoint, upstream.SuccessCount, upstream.FailedCount, upstream.LastCallSuccessTime,
				))
			}
			content := fmt.Sprintf("[%s] OpenAI 转发出错 ID: %d... 密钥: [%s] 上游: [%s] 错误: %s\n---\n%s",
				c.ClientIP(),
				upstream.ID, upstream.SK[:10], upstream.Endpoint, err.Error(),
				strings.Join(upstreamDescriptions, "\n"),
			)
			go sendMatrixMessage(content)
			if err.Error() != "context canceled" && r.Response.StatusCode != 400 {
				// count failed
				go db.Model(&upstream).Update("failed_count", gorm.Expr("failed_count + ?", 1))
				go sendFeishuMessage(content)
			}

			log.Println("response is", r.Response)
		}
		proxy.ServeHTTP(c.Writer, c.Request)
		resp, err := io.ReadAll(io.NopCloser(&buf))
		if err != nil {
			log.Println("Failed to read from response tee buffer", err)
		}
		go recordAssistantResponse(contentType, db, trackID, resp)
	})

	// ---------------------------------
	// admin APIs
	engine.POST("/admin/login", func(c *gin.Context) {
		// check authorization headers
		if handleAuth(c) != nil {
			return
		}
		c.JSON(200, gin.H{
			"message": "success",
		})
	})
	engine.GET("/admin/upstreams", func(c *gin.Context) {
		// check authorization headers
		if handleAuth(c) != nil {
			return
		}
		upstreams := make([]OPENAI_UPSTREAM, 0)
		db.Find(&upstreams)
		c.JSON(200, upstreams)
	})
	engine.POST("/admin/upstreams", func(c *gin.Context) {
		// check authorization headers
		if handleAuth(c) != nil {
			return
		}
		newUpstream := OPENAI_UPSTREAM{}
		err := c.BindJSON(&newUpstream)
		if err != nil {
			c.AbortWithError(502, errors.New("can't parse OPENAI_UPSTREAM object"))
			return
		}
		if newUpstream.SK == "" || newUpstream.Endpoint == "" {
			c.AbortWithError(403, errors.New("can't create new OPENAI_UPSTREAM with empty sk or endpoint"))
			return
		}
		log.Println("Saveing new OPENAI_UPSTREAM", newUpstream)
		err = db.Create(&newUpstream).Error
		if err != nil {
			c.AbortWithError(403, err)
			return
		}
	})
	engine.DELETE("/admin/upstreams/:id", func(ctx *gin.Context) {
		// check authorization headers
		if handleAuth(ctx) != nil {
			return
		}
		id, err := strconv.Atoi(ctx.Param("id"))
		if err != nil {
			ctx.AbortWithError(502, err)
			return
		}
		upstream := OPENAI_UPSTREAM{}
		upstream.ID = uint(id)
		db.Delete(&upstream)
		ctx.JSON(200, gin.H{
			"message": "success",
		})
	})
	engine.PUT("/admin/upstreams/:id", func(c *gin.Context) {
		// check authorization headers
		if handleAuth(c) != nil {
			return
		}
		upstream := OPENAI_UPSTREAM{}
		err := c.BindJSON(&upstream)
		if err != nil {
			c.AbortWithError(502, errors.New("can't parse OPENAI_UPSTREAM object"))
			return
		}
		if upstream.SK == "" || upstream.Endpoint == "" {
			c.AbortWithError(403, errors.New("can't create new OPENAI_UPSTREAM with empty sk or endpoint"))
			return
		}
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.AbortWithError(502, err)
			return
		}
		upstream.ID = uint(id)
		log.Println("Saveing new OPENAI_UPSTREAM", upstream)
		err = db.Create(&upstream).Error
		if err != nil {
			c.AbortWithError(403, err)
			return
		}
		c.JSON(200, gin.H{
			"message": "success",
		})
	})
	engine.GET("/admin/request_records", func(c *gin.Context) {
		// check authorization headers
		if handleAuth(c) != nil {
			return
		}
		requestRecords := []Record{}
		err := db.Order("id desc").Limit(100).Find(&requestRecords).Error
		if err != nil {
			c.AbortWithError(502, err)
			return
		}
		c.JSON(200, requestRecords)
	})
	engine.Run(*listenAddr)
}
