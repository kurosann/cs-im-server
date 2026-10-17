package api

import (
	"net/http"

	"cs-im-server/pkg/httpapi"
)

type ConversationAPI struct{}

func (a *ConversationAPI) Create(ctx httpapi.Context) {
	httpapi.WriteError(ctx, http.StatusNotImplemented, "NOT_IMPLEMENTED", "conversation create entrypoint reserved")
}

func (a *ConversationAPI) List(ctx httpapi.Context) {
	httpapi.WriteError(ctx, http.StatusNotImplemented, "NOT_IMPLEMENTED", "conversation list entrypoint reserved")
}

func (a *ConversationAPI) Get(ctx httpapi.Context) {
	httpapi.WriteError(ctx, http.StatusNotImplemented, "NOT_IMPLEMENTED", "conversation get entrypoint reserved")
}

func (a *ConversationAPI) Assign(ctx httpapi.Context) {
	httpapi.WriteError(ctx, http.StatusNotImplemented, "NOT_IMPLEMENTED", "conversation assign entrypoint reserved")
}

func (a *ConversationAPI) Close(ctx httpapi.Context) {
	httpapi.WriteError(ctx, http.StatusNotImplemented, "NOT_IMPLEMENTED", "conversation close entrypoint reserved")
}

func (a *ConversationAPI) Forward(ctx httpapi.Context) {
	httpapi.WriteError(ctx, http.StatusNotImplemented, "NOT_IMPLEMENTED", "conversation forward entrypoint reserved")
}

func (a *ConversationAPI) Messages(ctx httpapi.Context) {
	httpapi.WriteError(ctx, http.StatusNotImplemented, "NOT_IMPLEMENTED", "conversation messages entrypoint reserved")
}

func (a *ConversationAPI) Deliveries(ctx httpapi.Context) {
	httpapi.WriteError(ctx, http.StatusNotImplemented, "NOT_IMPLEMENTED", "conversation deliveries entrypoint reserved")
}
