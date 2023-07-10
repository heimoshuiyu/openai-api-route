package main

import (
	"time"

	"gorm.io/gorm"
)

// one openai upstream contain a pair of key and endpoint
type OPENAI_UPSTREAM struct {
	gorm.Model
	SK                  string `gorm:"index:idx_sk_endpoint,unique"` // key
	Endpoint            string `gorm:"index:idx_sk_endpoint,unique"` // endpoint
	SuccessCount        int64
	FailedCount         int64
	LastCallSuccessTime time.Time
}
