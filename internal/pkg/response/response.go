package response

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog/log"
)

// APIResponse is the standard envelope returned by every endpoint.
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *APIError   `json:"error,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
}

// APIError describes a single error returned to the client.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Meta carries pagination / extra information.
type Meta struct {
	Page  int64 `json:"page,omitempty"`
	Limit int64 `json:"limit,omitempty"`
	Total int64 `json:"total,omitempty"`
}

// WriteJSON writes a success response with the given status code.
func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	write(w, status, APIResponse{Success: true, Data: data})
}

// WriteOK is a convenience helper for 200 responses.
func WriteOK(w http.ResponseWriter, data interface{}) {
	WriteJSON(w, http.StatusOK, data)
}

// WriteMeta writes a success response including pagination metadata.
func WriteMeta(w http.ResponseWriter, data interface{}, meta Meta) {
	write(w, http.StatusOK, APIResponse{Success: true, Data: data, Meta: &meta})
}

// WriteError writes a standard error response.
func WriteError(w http.ResponseWriter, status int, code, message string) {
	write(w, status, APIResponse{Success: false, Error: &APIError{Code: code, Message: message}})
}

// WriteCreated writes a 201 response.
func WriteCreated(w http.ResponseWriter, data interface{}) {
	write(w, http.StatusCreated, APIResponse{Success: true, Data: data})
}

func write(w http.ResponseWriter, status int, payload APIResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Error().Err(err).Msg("failed to encode response")
	}
}
