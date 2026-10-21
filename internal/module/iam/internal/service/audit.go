package service

import (
	"context"
	iamprincipal "cs-im-server/internal/module/iam/pkg/principal"
	"strings"

	iamdto "cs-im-server/internal/module/iam/dto"
	"cs-im-server/internal/module/iam/internal/model"

	"vane-common/pkg/dataaccess"
)

type AuditContextKey struct{}

type AuditService struct {
	logs dataaccess.DataAccessor[*model.AuditLog]
}

func NewAuditService() *AuditService {
	return &AuditService{}
}

func WithAuditContext(ctx context.Context, requestContext iamdto.AuditContext) context.Context {
	return context.WithValue(ctx, AuditContextKey{}, requestContext)
}

func AuditContextFromContext(ctx context.Context) (iamdto.AuditContext, bool) {
	requestContext, ok := ctx.Value(AuditContextKey{}).(iamdto.AuditContext)
	return requestContext, ok
}

func (s *AuditService) Record(ctx context.Context, principal iamprincipal.Principal, entry iamdto.AuditEntry) {
	if principal.TenantID == "" || entry.Event == "" || entry.Action == "" {
		return
	}
	result := entry.Result
	if result == "" {
		result = "success"
	}
	requestContext, _ := AuditContextFromContext(ctx)
	item := &model.AuditLog{
		TenantID:  principal.TenantID,
		ActorKey:  principal.AccountKey,
		ActorName: principal.Name,
		ActorKind: principal.Kind,
		Event:     entry.Event,
		Action:    entry.Action,
		Object:    entry.Object,
		Result:    result,
		IP:        requestContext.IP,
		Device:    requestContext.Device,
	}
	_ = s.logs.Create(ctx, item)
}

func (s *AuditService) List(ctx context.Context, principal iamprincipal.Principal, filter iamdto.AuditFilter) (iamdto.AuditPage, error) {
	if principal.Kind != iamprincipal.KindHuman || !principal.HasScope(ScopeAuditRead) {
		return iamdto.AuditPage{}, forbidden("principal cannot read audit logs")
	}
	conditions := []dataaccess.QueryCondition{
		dataaccess.Eq("tenant_id", principal.TenantID),
		dataaccess.Order("create_at", dataaccess.OrderTypeDesc),
	}
	conditions = append(conditions, dataaccess.Page(filter.Page, filter.PageSize)...)
	countConditions := []dataaccess.QueryCondition{
		dataaccess.Eq("tenant_id", principal.TenantID),
	}
	if action := strings.TrimSpace(filter.Action); action != "" {
		conditions = append(conditions, dataaccess.Eq("action", action))
		countConditions = append(countConditions, dataaccess.Eq("action", action))
	}
	if result := strings.TrimSpace(filter.Result); result != "" && result != "all" {
		conditions = append(conditions, dataaccess.Eq("result", result))
		countConditions = append(countConditions, dataaccess.Eq("result", result))
	}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		pattern := "%" + keyword + "%"
		search := dataaccess.Or(
			dataaccess.Like("action", pattern),
			dataaccess.Like("event", pattern),
			dataaccess.Like("object", pattern),
			dataaccess.Like("actor_key", pattern),
			dataaccess.Like("actor_name", pattern),
		)
		conditions = append(conditions, search)
		countConditions = append(countConditions, search)
	}
	items, err := s.logs.List(ctx, conditions...)
	if err != nil {
		return iamdto.AuditPage{}, err
	}
	total, err := s.logs.Count(ctx, countConditions...)
	if err != nil {
		return iamdto.AuditPage{}, err
	}
	result := iamdto.AuditPage{Items: make([]iamdto.AuditLog, 0, len(items)), Total: total}
	for _, item := range items {
		result.Items = append(result.Items, iamdto.AuditLog{
			ID:        item.ID,
			Event:     item.Event,
			Action:    item.Action,
			Operator:  item.ActorName,
			Object:    item.Object,
			Result:    item.Result,
			IP:        item.IP,
			Device:    item.Device,
			CreatedAt: item.CreateAt.Format("2006-01-02 15:04:05"),
		})
	}
	return result, nil
}
