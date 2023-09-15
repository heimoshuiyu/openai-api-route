package main

import (
	"github.com/gin-gonic/gin"
)

// Middleware function to handle CORS requests
func handleCORS(c *gin.Context) {
	if c.Request.Method == "OPTIONS" {
		header := c.Writer.Header()
		header.Set("Access-Control-Allow-Origin", "*")
		header.Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, PATCH")
		header.Set("Access-Control-Allow-Headers", "Origin, Authorization, Content-Type")

		c.AbortWithStatus(200)
		return
	}

	c.Next()

	header := c.Writer.Header()

	header.Del("Access-Control-Allow-Origin")
	header.Del("Access-Control-Allow-Methods")
	header.Del("Access-Control-Allow-Headers")

	header.Set("Access-Control-Allow-Origin", "*")
	header.Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, PATCH")
	header.Set("Access-Control-Allow-Headers", "Origin, Authorization, Content-Type")

}
