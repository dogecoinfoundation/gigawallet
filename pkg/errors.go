package giga

import (
	"fmt"
)

type ErrorCode string

const (
	BadRequest    ErrorCode = "bad-request"
	NotAvailable  ErrorCode = "not-available"
	NotFound      ErrorCode = "not-found"
	AlreadyExists ErrorCode = "already-exists"
	UnknownError  ErrorCode = "unknown-error"
)

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

func IsNotFoundError(err error) bool {
	if e, ok := err.(*ErrorInfo); ok {
		return e.Code == NotFound
	}
	return false
}

func IsAlreadyExistsError(err error) bool {
	if e, ok := err.(*ErrorInfo); ok {
		return e.Code == AlreadyExists
	}
	return false
}
