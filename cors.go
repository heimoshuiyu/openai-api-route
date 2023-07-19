package main

import (
	"github.com/gin-gonic/gin"
)

// Middleware function to handle CORS requests
func handleCORS(c *gin.Context) {
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, PATCH")
	c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Authorization, Content-Type")

	if c.Request.Method == "OPTIONS" {
		c.AbortWithStatus(200)
		return
	}

}
