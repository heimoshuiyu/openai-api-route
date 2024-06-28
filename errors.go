package main

import "errors"

var (
	ErrReadRequestBody = errors.New("failed to read request body")
)
