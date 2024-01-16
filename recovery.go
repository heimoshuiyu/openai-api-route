package main

import (
	"errors"
	"log"
	"net/http"
	"net/http/httputil"

	"github.com/gin-gonic/gin"
)

func ServeHTTP(proxy *httputil.ReverseProxy, w gin.ResponseWriter, r *http.Request) (errReturn error) {

	// recovery
	defer func() {
		if err := recover(); err != nil {
			log.Println("[serve.panic]: ", err)
			errReturn = errors.New("[serve.panic]: Panic recover in reverse proxy serve HTTP")
		}
	}()

	proxy.ServeHTTP(w, r)
	return nil
}
