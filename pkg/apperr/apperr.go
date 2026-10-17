package apperr

import (
	"errors"
	"fmt"
)

type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string {
	return e.Message
}

func New(status int, code, format string, args ...any) *Error {
	return &Error{Status: status, Code: code, Message: fmt.Sprintf(format, args...)}
}

func BadRequest(format string, args ...any) *Error {
	return New(400, "BAD_REQUEST", format, args...)
}

func Unauthorized(format string, args ...any) *Error {
	return New(401, "UNAUTHORIZED", format, args...)
}

func Forbidden(format string, args ...any) *Error {
	return New(403, "FORBIDDEN", format, args...)
}

func NotFound(format string, args ...any) *Error {
	return New(404, "NOT_FOUND", format, args...)
}

func From(err error) (*Error, bool) {
	var target *Error
	if errors.As(err, &target) {
		return target, true
	}
	return nil, false
}
