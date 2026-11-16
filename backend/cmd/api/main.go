package main

import (
	"energyhub-universal/backend/internal/platform/httpapi"
	"log"
	"net/http"
	"os"
)

func main() {
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Fatal(http.ListenAndServe(addr, httpapi.NewRouter()))
}
