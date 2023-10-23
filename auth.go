package main

import (
	"errors"
	"log"
	"strings"

	"github.com/gin-gonic/gin"
)

func handleAuth(c *gin.Context) error {
	var err error

	authorization := c.Request.Header.Get("Authorization")
	if !strings.HasPrefix(authorization, "Bearer") {
		err = errors.New("authorization header should start with 'Bearer'")
		c.AbortWithError(403, err)
		return err
	}

	authorization = strings.Trim(authorization[len("Bearer"):], " ")
	log.Println("Received authorization", authorization)

	if authorization != authConfig.Value {
		err = errors.New("wrong authorization header")
		c.AbortWithError(403, err)
		return err
	}

	return nil
}
