package main

import (
	"log"
	"net/url"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Address       string            `yaml:"address"`
	Hostname      string            `yaml:"hostname"`
	DBType        string            `yaml:"dbtype"`
	DBAddr        string            `yaml:"dbaddr"`
	Authorization string            `yaml:"authorization"`
	Timeout       int64             `yaml:"timeout"`
	StreamTimeout int64             `yaml:"stream_timeout"`
	Upstreams     []OPENAI_UPSTREAM `yaml:"upstreams"`
	Random        bool              `yaml:"random"`
	CliConfig     CliConfig
}

type CliConfig struct {
	ConfigFile string
	ListMode   bool
	Noauth     bool
	DBLog      bool
}

func ReadConfig(filepath string) Config {
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
		log.Println("Address not set, use default value: :8888")
		config.Address = ":8888"
	}
	if config.DBType == "" {
		log.Println("DBType not set, use default value: sqlite")
		config.DBType = "sqlite"
	}
	if config.DBAddr == "" {
		log.Println("DBAddr not set, use default value: ./db.sqlite")
		config.DBAddr = "./db.sqlite"
	}
	if config.Timeout == 0 {
		log.Println("Timeout not set, use default value: 120")
		config.Timeout = 120
	}
	if config.StreamTimeout == 0 {
		log.Println("StreamTimeout not set, use default value: 10")
		config.StreamTimeout = 10
	}

	for i, upstream := range config.Upstreams {
		// parse upstream endpoint URL
		endpoint, err := url.Parse(upstream.Endpoint)
		if err != nil {
			log.Fatalf("Can't parse upstream endpoint URL '%s': %s", upstream.Endpoint, err)
		}
		config.Upstreams[i].URL = endpoint
		if config.Upstreams[i].Type == "" {
			config.Upstreams[i].Type = "openai"
		}
		if (config.Upstreams[i].Type != "openai") && (config.Upstreams[i].Type != "replicate") {
			log.Fatalf("Unsupported upstream type '%s'", config.Upstreams[i].Type)
		}
		// apply authorization from global config if not set
		if config.Upstreams[i].Authorization == "" && !config.Upstreams[i].Noauth {
			config.Upstreams[i].Authorization = config.Authorization
		}
		if config.Upstreams[i].Timeout == 0 {
			config.Upstreams[i].Timeout = config.Timeout
		}
		if config.Upstreams[i].StreamTimeout == 0 {
			config.Upstreams[i].StreamTimeout = config.StreamTimeout
		}
	}

	return config
}
