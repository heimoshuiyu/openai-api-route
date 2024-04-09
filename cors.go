package main

import (
	"log"

	"github.com/gin-gonic/gin"
)

func sendCORSHeaders(c *gin.Context) {
	log.Println("sendCORSHeaders")
	if c.Writer.Header().Get("Access-Control-Allow-Origin") == "" {
		c.Header("Access-Control-Allow-Origin", "*")
	}
	if c.Writer.Header().Get("Access-Control-Allow-Methods") == "" {
		c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, PATCH")
	}
	if c.Writer.Header().Get("Access-Control-Allow-Headers") == "" {
		c.Header("Access-Control-Allow-Headers", "Origin, Authorization, Content-Type")
	}
}
