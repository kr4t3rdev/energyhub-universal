package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
)

func TestRequestIDMiddlewareAddsHeaderAndContext(t *testing.T) {
	next := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestID, ok := RequestIDFromContext(request.Context())
		if !ok {
			t.Fatal("request ID missing from context")
		}
		if requestID == "" {
			t.Fatal("request ID is empty")
		}
		writer.WriteHeader(http.StatusNoContent)
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	RequestIDMiddleware(next).ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", response.Code)
	}
	requestID := response.Header().Get(RequestIDHeader)
	if !regexp.MustCompile(`^[a-f0-9]{32}$`).MatchString(requestID) {
		t.Fatalf("request ID = %q, want 32 lowercase hexadecimal characters", requestID)
	}
}

func TestRequestIDFromContextReportsMissingOrNilContext(t *testing.T) {
	if _, ok := RequestIDFromContext(context.Background()); ok {
		t.Fatal("request ID unexpectedly present")
	}
	if _, ok := RequestIDFromContext(nil); ok {
		t.Fatal("request ID unexpectedly present for nil context")
	}
}

func TestRequestIDMiddlewareReturnsServerErrorWhenGeneratorFails(t *testing.T) {
	nextCalls := 0
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		nextCalls++
	})
	response := httptest.NewRecorder()

	requestIDMiddleware(next, func() (string, error) {
		return "", errors.New("random source unavailable")
	}).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", response.Code)
	}
	if nextCalls != 0 {
		t.Fatalf("next calls = %d, want 0", nextCalls)
	}
}

func TestRequestIDMiddlewareReturnsServerErrorWhenGeneratorIsNil(t *testing.T) {
	response := httptest.NewRecorder()

	requestIDMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), nil).
		ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", response.Code)
	}
}
