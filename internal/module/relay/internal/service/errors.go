package service

import "cs-im-server/pkg/apperr"

type Error = apperr.Error

func badRequest(format string, args ...any) *Error { return apperr.BadRequest(format, args...) }

func notFound(format string, args ...any) *Error { return apperr.NotFound(format, args...) }

func unauthorized(format string, args ...any) *Error { return apperr.Unauthorized(format, args...) }

func forbidden(format string, args ...any) *Error { return apperr.Forbidden(format, args...) }

func conflict(format string, args ...any) *Error { return apperr.New(409, "CONFLICT", format, args...) }
