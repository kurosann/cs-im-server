package api

import (
	"net/http"

	"cs-im-server/pkg/httpapi"
)

type MessageAPI struct{}

func (a *MessageAPI) Create(ctx httpapi.Context) {
	httpapi.WriteError(ctx, http.StatusNotImplemented, "NOT_IMPLEMENTED", "message create entrypoint reserved")
}

func (a *MessageAPI) UpdateStatus(ctx httpapi.Context) {
	httpapi.WriteError(ctx, http.StatusNotImplemented, "NOT_IMPLEMENTED", "message status update entrypoint reserved")
}
