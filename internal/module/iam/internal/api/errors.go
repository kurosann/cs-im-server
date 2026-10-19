package api

import (
	"cs-im-server/pkg/apperr"
	"cs-im-server/pkg/httpapi"
)

func writeServiceError(ctx httpapi.Context, err error) {
	if appErr, ok := apperr.From(err); ok {
		httpapi.WriteError(ctx, appErr.Status, appErr.Code, appErr.Message)
		return
	}
	httpapi.WriteError(ctx, 500, "INTERNAL_ERROR", err.Error())
}
