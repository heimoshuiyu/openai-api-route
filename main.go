package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/penglongli/gin-metrics/ginmetrics"
	"gorm.io/driver/postgres"
	"gorm.io/gorm/logger"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// global config
var config Config

func main() {
	configFile := flag.String("config", "./config.yaml", "Config file")
	listMode := flag.Bool("list", false, "List all upstream")
	noauth := flag.Bool("noauth", false, "Do not check incoming authorization header")
	dbLog := flag.Bool("dblog", false, "Enable database log")
	flag.Parse()

	log.Println("[main]: Service starting")

	// load all upstreams
	config = ReadConfig(*configFile)
	config.CliConfig = CliConfig{
		ConfigFile: *configFile,
		ListMode:   *listMode,
		Noauth:     *noauth,
		DBLog:      *dbLog,
	}
	log.Println("[main]: Load upstreams number:", len(config.Upstreams))

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
			log.Fatalf("[main]: Error to connect sqlite database: %s", err)
		}
	case "postgres":
		db, err = gorm.Open(postgres.Open(config.DBAddr), &gorm.Config{
			PrepareStmt:            true,
			SkipDefaultTransaction: true,
		})
		if err != nil {
			log.Fatalf("[main]: Error to connect postgres database: %s", err)
		}
	case "none":
		log.Println("[main]: No database connection")
	default:
		log.Fatalf("[main]: Unsupported database type: '%s'", config.DBType)
	}

	// init handler struct
	openAIAPI := OpenAIAPI{
		Config: &config,
		DB:     db,
	}

	if *dbLog && db != nil {
		db.Logger.LogMode(logger.Info)
	}

	db.AutoMigrate(&Record{})
	log.Println("[main]: Auto migrate database done")

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
	// engine.Use(corsMiddleware())

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
		// set cros header
		ctx.Header("Access-Control-Allow-Origin", "*")
		ctx.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, PATCH")
		ctx.Header("Access-Control-Allow-Headers", "Origin, Authorization, Content-Type")
		ctx.AbortWithStatus(200)
	})

	engine.POST("/v1/*any", openAIAPI.V1Handler)

	engine.Run(config.Address)
}
