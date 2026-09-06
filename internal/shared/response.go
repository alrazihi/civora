package shared

import (
	"encoding/json"
	"net/http"
)

type APIResponse struct {
	Success bool            `json:"success"`
	Data    any             `json:"data,omitempty"`
	Error   *ErrorResponse  `json:"error,omitempty"`
	Meta    *PaginationMeta `json:"meta,omitempty"`
}

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type PaginationMeta struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

func WriteJSON(w http.ResponseWriter, statusCode int, payload APIResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(payload)
}

func WriteError(w http.ResponseWriter, statusCode int, code ErrorCode, message string) {
	WriteJSON(w, statusCode, APIResponse{
		Success: false,
		Error: &ErrorResponse{
			Code:    string(code),
			Message: message,
		},
	})
}

func WriteSuccess(w http.ResponseWriter, statusCode int, data any, meta *PaginationMeta) {
	WriteJSON(w, statusCode, APIResponse{
		Success: true,
		Data:    data,
		Meta:    meta,
	})
}
