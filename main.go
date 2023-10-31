package main

import (
	"flag"
	"fmt"
	"log"
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

	// load all upstreams
	upstreams := make([]OPENAI_UPSTREAM, 0)
	db.Find(&upstreams)
	log.Println("Load upstreams number:", len(upstreams))

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

		for index, upstream := range upstreams {
			if upstream.Endpoint == "" || upstream.SK == "" {
				c.AbortWithError(500, fmt.Errorf("invaild upstream '%s' '%s'", upstream.SK, upstream.Endpoint))
				continue
			}

			shouldResponse := index == len(upstreams)-1

			err = processRequest(c, &upstream, &record, shouldResponse)
			if err != nil {
				log.Println("Error from upstream, should retry", upstream.SK, err)
				continue
			}

			break
		}

		log.Println("Record result:", record.Status, record.Response)
		record.ElapsedTime = time.Now().Sub(record.CreatedAt)
		if db.Create(&record).Error != nil {
			log.Println("Error to save record:", record)
		}
		if record.Status != 200 && record.Response != "context canceled" {
			errMessage := fmt.Sprintf("IP: %s request all upstreams error %d with %s", record.IP, record.Status, record.Response)
			go sendFeishuMessage(errMessage)
			go sendMatrixMessage(errMessage)
		}
	})

	engine.Run(*listenAddr)
}
