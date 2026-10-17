package service

import "fmt"

type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string {
	return e.Message
}

func badRequest(format string, args ...any) *Error {
	return &Error{Status: 400, Code: "BAD_REQUEST", Message: fmt.Sprintf(format, args...)}
}

func notFound(format string, args ...any) *Error {
	return &Error{Status: 404, Code: "NOT_FOUND", Message: fmt.Sprintf(format, args...)}
}
