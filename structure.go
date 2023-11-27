package main

import (
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
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

	return config
}
