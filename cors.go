package main

import (
	"github.com/gin-gonic/gin"
)

// this function is aborded
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// set cors header
		header := c.Request.Header
		if header.Get("Access-Control-Allow-Origin") == "" {
			c.Header("Access-Control-Allow-Origin", "*")
		}
		if header.Get("Access-Control-Allow-Methods") == "" {
			c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, PATCH")
		}
		if header.Get("Access-Control-Allow-Headers") == "" {
			c.Header("Access-Control-Allow-Headers", "Origin, Authorization, Content-Type")
		}
	}
}

func sendCORSHeaders(c *gin.Context) {
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, PATCH")
	c.Header("Access-Control-Allow-Headers", "Origin, Authorization, Content-Type")
}
