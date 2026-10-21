package http

import (
	"net/http"

	"cs-im-server/internal/module/relay/internal/service"
	"cs-im-server/pkg/httpapi"
	commonmodule "vane-common/module"
)

type KnowledgeAPI struct {
	Service *service.RelayService
}

func NewKnowledgeAPI(relayService *service.RelayService) *KnowledgeAPI {
	return &KnowledgeAPI{Service: relayService}
}

func (a *KnowledgeAPI) List(ctx httpapi.Context) {
	principal, ok := employeePrincipal(ctx)
	if !ok {
		return
	}
	library, err := a.Service.ListKnowledge(ctx.Context(), principal.TenantID)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", library)
}

func (a *KnowledgeAPI) CreateGroup(ctx httpapi.Context) {
	principal, ok := employeePrincipal(ctx)
	if !ok {
		return
	}
	var req service.CreateKnowledgeGroupRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", "请求格式不正确")
		return
	}
	item, err := a.Service.CreateKnowledgeGroup(ctx.Context(), principal.TenantID, req)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", item)
}

func (a *KnowledgeAPI) CreateBase(ctx httpapi.Context) {
	principal, ok := employeePrincipal(ctx)
	if !ok {
		return
	}
	var req service.CreateKnowledgeBaseRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", "请求格式不正确")
		return
	}
	item, err := a.Service.CreateKnowledgeBase(ctx.Context(), principal.TenantID, req)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", item)
}

func (a *KnowledgeAPI) CreateDoc(ctx httpapi.Context) {
	principal, ok := employeePrincipal(ctx)
	if !ok {
		return
	}
	var req service.SaveKnowledgeDocRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", "请求格式不正确")
		return
	}
	item, err := a.Service.CreateKnowledgeDoc(ctx.Context(), principal.TenantID, req)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", item)
}

func (a *KnowledgeAPI) UpdateDoc(ctx httpapi.Context) {
	principal, ok := employeePrincipal(ctx)
	if !ok {
		return
	}
	var req service.SaveKnowledgeDocRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", "请求格式不正确")
		return
	}
	item, err := a.Service.UpdateKnowledgeDoc(ctx.Context(), principal.TenantID, ctx.Param("id"), req)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", item)
}

func (a *KnowledgeAPI) Routes() commonmodule.HttpRouteGroup {
	routes := commonmodule.NewHttpRouteGroup("/knowledge")
	routes.Get("", a.List)
	routes.Post("/groups", a.CreateGroup)
	routes.Post("/bases", a.CreateBase)
	routes.Post("/docs", a.CreateDoc)
	routes.Patch("/docs/:id", a.UpdateDoc)
	return routes
}
