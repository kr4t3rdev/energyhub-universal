package httpapi

import (
	"encoding/json"
	"net/http"

	"energyhub-universal/backend/internal/contracts"
)

func WriteErrorResponse(writer http.ResponseWriter, status int, code, message, requestID string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(contracts.ErrorResponse{
		Code:      code,
		Message:   message,
		RequestID: requestID,
	})
}
