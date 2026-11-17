package httpapi

import (
	"github.com/go-chi/chi/v5"
	"net/http"
)

func NewRouter() http.Handler {
	r := chi.NewRouter()
	r.Use(RequestIDMiddleware)
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	return r
}
