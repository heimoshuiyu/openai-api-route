package main

import (
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

// one openai upstream contain a pair of key and endpoint
type OPENAI_UPSTREAM struct {
	SK       string `yaml:"sk"`
	Endpoint string `yaml:"endpoint"`
	Timeout  int64  `yaml:"timeout"`
}

func readUpstreams(filepath string) []OPENAI_UPSTREAM {
	var upstreams []OPENAI_UPSTREAM

	// read yaml file
	data, err := os.ReadFile(filepath)
	if err != nil {
		log.Fatalf("Error reading YAML file: %s", err)
	}

	// Unmarshal the YAML into the upstreams slice
	err = yaml.Unmarshal(data, &upstreams)
	if err != nil {
		log.Fatalf("Error unmarshaling YAML: %s", err)
	}

	return upstreams
}
