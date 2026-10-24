package grpc

import (
	"context"
	"net/http"

	"cs-im-server/pkg/apperr"
	"cs-im-server/pkg/httpapi"
	logger "vane-common/pkg/logger"
)

func debugContextRequest(ctx context.Context, format string, args ...any) {
	logger.Debugf(ctx, "channel grpc: "+format, args...)
}

func writeServiceError(ctx httpapi.Context, err error) {
	if serviceErr, ok := apperr.From(err); ok {
		httpapi.WriteError(ctx, serviceErr.Status, serviceErr.Code, serviceErr.Message)
		return
	}
	httpapi.WriteError(ctx, http.StatusInternalServerError, "INTERNAL", err.Error())
}
