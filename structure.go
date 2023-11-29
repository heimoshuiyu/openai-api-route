package main

import (
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Address       string            `yaml:"address"`
	DBType        string            `yaml:"dbtype"`
	DBAddr        string            `yaml:"dbaddr"`
	Authorization string            `yaml:"authorization"`
	Upstreams     []OPENAI_UPSTREAM `yaml:"upstreams"`
}
type OPENAI_UPSTREAM struct {
	SK       string `yaml:"sk"`
	Endpoint string `yaml:"endpoint"`
	Timeout  int64  `yaml:"timeout"`
}

func readConfig(filepath string) Config {
	var config Config

	// read yaml file
	data, err := os.ReadFile(filepath)
	if err != nil {
		log.Fatalf("Error reading YAML file: %s", err)
	}

	// Unmarshal the YAML into the upstreams slice
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		log.Fatalf("Error unmarshaling YAML: %s", err)
	}

	// set default value
	if config.Address == "" {
		config.Address = ":8888"
	}
	if config.DBType == "" {
		config.DBType = "sqlite"
	}
	if config.DBAddr == "" {
		config.DBAddr = "./db.sqlite"
	}

	return config
}
