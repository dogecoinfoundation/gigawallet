package giga

import (
	"fmt"
	"net/http"
)

type ErrorCode string

const (
	BadRequest    ErrorCode = "bad-request"
	NotAvailable  ErrorCode = "not-available"
	NotFound      ErrorCode = "not-found"
	AlreadyExists ErrorCode = "already-exists"
	UnknownError  ErrorCode = "unknown-error"
)

var httpCodeForError = map[string]int{
	string(BadRequest):    400,
	string(NotAvailable):  503,
	string(NotFound):      404,
	string(AlreadyExists): 500,
	string(UnknownError):  500,
}

type ErrorInfo struct {
	Code    ErrorCode // machine-readble ErrorCode enumeration
	Message string    // human-readable debug message (in production, logged on the server only)
}

func (e *ErrorInfo) Error() string {
	return string(e.Message)
}

func NewErr(code ErrorCode, format string, args ...any) error {
	return &ErrorInfo{Code: code, Message: fmt.Sprintf(format, args...)}
}

func HttpStatusForError(code ErrorCode) int {
	status, found := httpCodeForError[string(code)]
	if !found {
		status = http.StatusInternalServerError
	}
	return status
}
