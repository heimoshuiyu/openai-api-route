package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type RequestRecord struct {
	gorm.Model
	Body string
}

func recordUserMessage(c *gin.Context, db *gorm.DB, body []byte) {
	bodyStr := string(body)
	requestRecord := RequestRecord{
		Body: bodyStr,
	}
	err := db.Create(&requestRecord).Error
	if err != nil {
		log.Println("Error record request:", err)
	}
}
