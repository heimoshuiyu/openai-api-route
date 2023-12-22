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
		return err
	}

	authorization = strings.Trim(authorization[len("Bearer"):], " ")
	log.Println("Received authorization", authorization)

	for _, auth := range strings.Split(config.Authorization, ",") {
		if authorization != strings.Trim(auth, " ") {
			err = errors.New("wrong authorization header")
			return err
		}
	}

	return nil
}
