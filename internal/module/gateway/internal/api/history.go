package api

import (
	"cs-im-server/pkg/httpapi"
	"net/http"
	logger "vane-common/pkg/logger"
)

func (a *MessageAPI) RequestHistory(ctx httpapi.Context) {
	logger.Debugf(ctx.Context(), "gateway api %s %s: request conversation history id=%s", ctx.Request().Method, ctx.Request().URL.Path, ctx.Param("id"))
	request, err := a.Service.CreateHistoryRequest(ctx.Context(), ctx.Param("id"))
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", request)
}
