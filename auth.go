package main

import (
	"errors"
	"strings"
)

func checkAuth(authorization string, config string) error {
	for _, auth := range strings.Split(config, ",") {
		if authorization == strings.Trim(auth, " ") {
			return nil
		}
	}
	return errors.New("wrong authorization header")
}
