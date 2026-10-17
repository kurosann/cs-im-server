package grpcapi

import (
	"context"
	"strings"

	"cs-im-server/pkg/apperr"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func AccountToken(ctx context.Context) string {
	if value := metadataValue(ctx, "authorization"); value != "" {
		return strings.TrimPrefix(value, "Bearer ")
	}
	return metadataValue(ctx, "x-account-token")
}

func metadataValue(ctx context.Context, key string) string {
	values := metadata.ValueFromIncomingContext(ctx, key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func InvalidArgument(format string, args ...any) error {
	return status.Errorf(codes.InvalidArgument, format, args...)
}

func Error(err error) error {
	serviceErr, ok := apperr.From(err)
	if !ok {
		return status.Error(codes.Internal, err.Error())
	}
	code := codes.Internal
	switch serviceErr.Status {
	case 400:
		code = codes.InvalidArgument
	case 401:
		code = codes.Unauthenticated
	case 403:
		code = codes.PermissionDenied
	case 404:
		code = codes.NotFound
	}
	return status.Error(code, serviceErr.Message)
}
