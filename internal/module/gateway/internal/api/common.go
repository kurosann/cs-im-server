package api

import (
	"errors"
	"net/http"

	"cs-im-server/internal/module/gateway/internal/service"
	"cs-im-server/pkg/httpapi"
)

func writeServiceError(ctx httpapi.Context, err error) {
	var serviceErr *service.Error
	if errors.As(err, &serviceErr) {
		httpapi.WriteError(ctx, serviceErr.Status, serviceErr.Code, serviceErr.Message)
		return
	}
	httpapi.WriteError(ctx, http.StatusInternalServerError, "INTERNAL", err.Error())
}
