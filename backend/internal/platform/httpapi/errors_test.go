package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"energyhub-universal/backend/internal/contracts"
)

func TestWriteErrorResponseUsesContractEnvelope(t *testing.T) {
	response := httptest.NewRecorder()

	WriteErrorResponse(response, http.StatusUnauthorized, "unauthorized", "authentication required", "request-1")

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.Code)
	}
	if got := response.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content type = %q, want application/json", got)
	}

	var body contracts.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	want := contracts.ErrorResponse{
		Code:      "unauthorized",
		Message:   "authentication required",
		RequestID: "request-1",
	}
	if body != want {
		t.Fatalf("body = %#v, want %#v", body, want)
	}
}
