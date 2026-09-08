package shared

import (
	"encoding/json"
	"log"
	"net/http"
)

// WriteBody writes raw bytes to w after the status header has already been
// written. Write errors on an http.ResponseWriter are almost always
// client-disconnect I/O errors that cannot be recovered from, so the error
// is logged and dropped rather than silently ignored.
func WriteBody(w http.ResponseWriter, body []byte) {
	if _, err := w.Write(body); err != nil {
		log.Printf("write response body: %v", err)
	}
}

// WriteRawJSON writes a pre-serialized JSON payload. Like WriteBody, write
// errors on an http.ResponseWriter are unrecoverable I/O errors and are
// logged rather than silently ignored.
func WriteRawJSON(w http.ResponseWriter, statusCode int, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	WriteBody(w, body)
}

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
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		// Encoding to an http.ResponseWriter can only fail on I/O errors
		// (client disconnect, etc.). Nothing meaningful can be done at this
		// point; the headers and status code have already been sent.
		_ = err
	}
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

func WritePaginatedSuccess(w http.ResponseWriter, statusCode int, data any, page, perPage, total int) {
	WriteSuccess(w, statusCode, data, &PaginationMeta{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: paginate(page, perPage, total),
	})
}

func paginate(page, perPage, total int) int {
	if perPage <= 0 {
		return 0
	}
	if total == 0 {
		return 0
	}
	pages := total / perPage
	if total%perPage > 0 {
		pages++
	}
	return pages
}
