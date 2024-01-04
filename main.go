package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/penglongli/gin-metrics/ginmetrics"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// global config
var config Config

func main() {
	configFile := flag.String("config", "./config.yaml", "Config file")
	listMode := flag.Bool("list", false, "List all upstream")
	noauth := flag.Bool("noauth", false, "Do not check incoming authorization header")
	flag.Parse()

	log.Println("Service starting")

	// load all upstreams
	config = readConfig(*configFile)
	log.Println("Load upstreams number:", len(config.Upstreams))

	// connect to database
	var db *gorm.DB
	var err error
	switch config.DBType {
	case "sqlite":
		db, err = gorm.Open(sqlite.Open(config.DBAddr), &gorm.Config{
			PrepareStmt:            true,
			SkipDefaultTransaction: true,
		})
		if err != nil {
			log.Fatalf("Error to connect sqlite database: %s", err)
		}
	case "postgres":
		db, err = gorm.Open(postgres.Open(config.DBAddr), &gorm.Config{
			PrepareStmt:            true,
			SkipDefaultTransaction: true,
		})
		if err != nil {
			log.Fatalf("Error to connect postgres database: %s", err)
		}
	default:
		log.Fatalf("Unsupported database type: '%s'", config.DBType)
	}

	db.AutoMigrate(&Record{})
	log.Println("Auto migrate database done")

	if *listMode {
		fmt.Println("SK\tEndpoint")
		for _, upstream := range config.Upstreams {
			fmt.Println(upstream.SK, upstream.Endpoint)
		}
		return
	}

	// init gin
	engine := gin.Default()

	// metrics
	m := ginmetrics.GetMonitor()
	m.SetMetricPath("/v1/metrics")
	m.Use(engine)

	// CORS middleware
	engine.Use(corsMiddleware())

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
		ctx.AbortWithStatus(200)
	})

	engine.POST("/v1/*any", func(c *gin.Context) {
		hostname, err := os.Hostname()
		if config.Hostname != "" {
			hostname = config.Hostname
		}
		record := Record{
			IP:            c.ClientIP(),
			Hostname:      hostname,
			CreatedAt:     time.Now(),
			Authorization: c.Request.Header.Get("Authorization"),
			UserAgent:     c.Request.Header.Get("User-Agent"),
		}
		/*
			defer func() {
				if err := recover(); err != nil {
					log.Println("Error:", err)
					c.AbortWithError(500, fmt.Errorf("%s", err))
				}
			}()
		*/

		// check authorization header
		if !*noauth {
			err := handleAuth(c)
			if err != nil {
				c.Header("Content-Type", "application/json")
				c.AbortWithError(403, err)
				return
			}
		}

		for index, upstream := range config.Upstreams {
			if upstream.Endpoint == "" || upstream.SK == "" {
				c.AbortWithError(500, fmt.Errorf("invaild upstream '%s' '%s'", upstream.SK, upstream.Endpoint))
				continue
			}

			shouldResponse := index == len(config.Upstreams)-1

			if len(config.Upstreams) == 1 {
				upstream.Timeout = 120
			}

			err = processRequest(c, &upstream, &record, shouldResponse)
			if err != nil {
				log.Println("Error from upstream", upstream.Endpoint, "should retry", err)
				continue
			}

			break
		}

		log.Println("Record result:", record.Status, record.Response)
		record.ElapsedTime = time.Now().Sub(record.CreatedAt)

		// async record request
		go func() {
			// turncate request if too long
			if len(record.Body) > 1024*128 {
				log.Println("Warning: Truncate request body")
				record.Body = record.Body[:1024*128]
			}
			if db.Create(&record).Error != nil {
				log.Println("Error to save record:", record)
			}
		}()

		if record.Status != 200 {
			errMessage := fmt.Sprintf("IP: %s request %s error %d with %s", record.IP, record.Model, record.Status, record.Response)
			go sendFeishuMessage(errMessage)
			go sendMatrixMessage(errMessage)
		}
	})

	engine.Run(config.Address)
}
