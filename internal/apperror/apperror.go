package apperror

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Code represents a machine-readable error code.
type Code string

const (
	// Client errors
	CodeBadRequest     Code = "BAD_REQUEST"
	CodeUnauthorized   Code = "UNAUTHORIZED"
	CodeForbidden      Code = "FORBIDDEN"
	CodeNotFound       Code = "NOT_FOUND"
	CodeMethodNotAllow Code = "METHOD_NOT_ALLOWED"
	CodeConflict       Code = "CONFLICT"
	CodeTooManyReqs    Code = "TOO_MANY_REQUESTS"
	CodeQuotaExceeded  Code = "QUOTA_EXCEEDED"
	CodePayloadTooLrg  Code = "PAYLOAD_TOO_LARGE"

	// Validation errors
	CodeMissingField   Code = "MISSING_FIELD"
	CodeInvalidField   Code = "INVALID_FIELD"
	CodeMessageEmpty   Code = "MESSAGES_REQUIRED"
	CodeMessageTooMany Code = "TOO_MANY_MESSAGES"
	CodeMessageTooLong Code = "MESSAGE_TOO_LONG"

	// Server errors
	CodeInternal       Code = "INTERNAL_ERROR"
	CodeLLMError       Code = "LLM_ERROR"
	CodeLLMUnavailable Code = "LLM_UNAVAILABLE"
	CodeSandboxError   Code = "SANDBOX_ERROR"
	CodeStorageError   Code = "STORAGE_ERROR"
)

// httpStatus maps error codes to HTTP status codes.
var httpStatus = map[Code]int{
	CodeBadRequest:     http.StatusBadRequest,
	CodeUnauthorized:   http.StatusUnauthorized,
	CodeForbidden:      http.StatusForbidden,
	CodeNotFound:       http.StatusNotFound,
	CodeMethodNotAllow: http.StatusMethodNotAllowed,
	CodeConflict:       http.StatusConflict,
	CodeTooManyReqs:    http.StatusTooManyRequests,
	CodeQuotaExceeded:  http.StatusTooManyRequests,
	CodePayloadTooLrg:  http.StatusRequestEntityTooLarge,
	CodeMissingField:   http.StatusBadRequest,
	CodeInvalidField:   http.StatusBadRequest,
	CodeMessageEmpty:   http.StatusBadRequest,
	CodeMessageTooMany: http.StatusBadRequest,
	CodeMessageTooLong: http.StatusBadRequest,
	CodeInternal:       http.StatusInternalServerError,
	CodeLLMError:       http.StatusBadGateway,
	CodeLLMUnavailable: http.StatusServiceUnavailable,
	CodeSandboxError:   http.StatusInternalServerError,
	CodeStorageError:   http.StatusInternalServerError,
}

// Error is a structured application error.
type Error struct {
	Code    Code   `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

func (e *Error) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Detail)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// HTTPStatus returns the corresponding HTTP status code.
func (e *Error) HTTPStatus() int {
	if s, ok := httpStatus[e.Code]; ok {
		return s
	}
	return http.StatusInternalServerError
}

// New creates a new AppError.
func New(code Code, message string) *Error {
	return &Error{Code: code, Message: message}
}

// Wrap creates an AppError with detail from an underlying error.
func Wrap(code Code, message string, err error) *Error {
	return &Error{Code: code, Message: message, Detail: err.Error()}
}

// Write sends a structured JSON error response.
func Write(w http.ResponseWriter, e *Error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(e.HTTPStatus())
	json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{
			"code":    e.Code,
			"message": e.Message,
			"detail":  e.Detail,
		},
	})
}

// WriteCode is a convenience for simple error responses.
func WriteCode(w http.ResponseWriter, code Code, message string) {
	Write(w, New(code, message))
}
