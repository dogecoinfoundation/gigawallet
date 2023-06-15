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
	DBConflict    ErrorCode = "db-conflict"
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
	return IsError(err, NotFound)
}

func IsAlreadyExistsError(err error) bool {
	return IsError(err, AlreadyExists)
}

func IsDBConflictError(err error) bool {
	return IsError(err, DBConflict)
}

func IsError(err error, ofType ErrorCode) bool {
	if e, ok := err.(*ErrorInfo); ok {
		return e.Code == ofType
	}
	return false
}
