package main

import (
	"errors"
	"log"

	"gorm.io/gorm"
)

// K-V struct to store program's config
type ConfigKV struct {
	gorm.Model
	Key   string `gorm:"unique"`
	Value string
}

// init db
func initconfig(db *gorm.DB) error {
	var err error

	err = db.AutoMigrate(&ConfigKV{})
	if err != nil {
		return err
	}

	// config list and their default values
	configs := make(map[string]string)
	configs["authorization"] = "woshimima"
	configs["policy"] = "random"

	for key, value := range configs {
		kv := ConfigKV{}
		err = db.Take(&kv, "key = ?", key).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				log.Println("Missing config", key, "creating with value", value)
				kv.Key = key
				kv.Value = value
				if err = db.Create(&kv).Error; err != nil {
					return err
				}
			} else {
				return err
			}
		}
	}

	return nil
}
