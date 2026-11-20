package handler

import (
	"context"
	"log"
	"net/http"
	"os"
	"sync"

	"energyhub-universal/backend/apiapp"
)

var (
	handlerMu  sync.Mutex
	apiHandler http.Handler
)

func Handler(w http.ResponseWriter, r *http.Request) {
	handlerMu.Lock()
	if apiHandler == nil {
		config, err := apiapp.LoadConfig(os.Getenv)
		if err != nil {
			handlerMu.Unlock()
			log.Printf("initialize API handler: %v", err)
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}
		var cleanup func()
		apiHandler, cleanup, err = apiapp.BuildRouter(context.Background(), config)
		if err != nil {
			handlerMu.Unlock()
			log.Printf("initialize API handler: %v", err)
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}
		// The database pool lives for the lifetime of the warm function instance.
		_ = cleanup
	}
	currentHandler := apiHandler
	handlerMu.Unlock()
	currentHandler.ServeHTTP(w, r)
}
