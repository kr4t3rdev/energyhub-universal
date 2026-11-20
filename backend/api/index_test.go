package handler

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestHandlerExposesHealthWithoutAuthConfig(t *testing.T) {
	t.Setenv("AUTH0_ISSUER", "")
	t.Setenv("AUTH0_AUDIENCE", "")
	t.Setenv("DATABASE_URL", "")

	handlerMu = sync.Mutex{}
	apiHandler = nil
	t.Cleanup(func() {
		handlerMu = sync.Mutex{}
		apiHandler = nil
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	Handler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("Handler(/health) status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
}
