package service

import (
	"context"
	"time"

	"vane-common/pkg/dataaccess"
	"vane-common/pkg/idutil"

	gatewaydto "cs-im-server/internal/module/gateway/dto"
	"cs-im-server/internal/module/gateway/internal/model"
	iampkg "cs-im-server/internal/module/iam/pkg/iam"
	iamprincipal "cs-im-server/internal/module/iam/pkg/principal"
)

type AppService struct {
	accessor dataaccess.DataAccessor[*model.App]
	realtime *RealtimeService
}

func NewAppService(realtime *RealtimeService) *AppService {
	return &AppService{realtime: realtime}
}

func (s *AppService) Register(_ context.Context, principal iamprincipal.Principal, req gatewaydto.RegisterAppRequest) (gatewaydto.App, error) {
	if req.ServiceKey == "" {
		return gatewaydto.App{}, badRequest("serviceKey is required")
	}
	if req.DisplayName == "" {
		return gatewaydto.App{}, badRequest("displayName is required")
	}

	now := time.Now().UTC()
	existing, err := s.GetByTenantAndService(context.Background(), principal.TenantID, req.ServiceKey)
	if err != nil {
		return gatewaydto.App{}, err
	}
	if existing != nil {
		app := existing
		app.DisplayName = req.DisplayName
		app.Kind = req.Kind
		app.Version = req.Version
		app.Capabilities = cloneStrings(req.Capabilities)
		app.UpdateAt = now
		if err := s.accessor.Save(context.Background(), app); err != nil {
			return gatewaydto.App{}, err
		}
		return appToDTO(app), nil
	}

	id := "app_" + idutil.GenShortId()
	app := &model.App{
		AppKey:          id,
		TenantID:        principal.TenantID,
		ServiceKey:      req.ServiceKey,
		DisplayName:     req.DisplayName,
		Kind:            req.Kind,
		Version:         req.Version,
		Capabilities:    cloneStrings(req.Capabilities),
		Status:          "active",
		LeaseTTLSeconds: 60,
	}
	if req.LeaseTTLSeconds > 0 {
		app.LeaseTTLSeconds = req.LeaseTTLSeconds
	}
	if err := s.accessor.Save(context.Background(), app); err != nil {
		return gatewaydto.App{}, err
	}
	result := *app
	s.realtime.Publish("app.registered", id, appToDTO(&result))
	return appToDTO(&result), nil
}

func (s *AppService) List(_ context.Context, principal iamprincipal.Principal) []model.App {
	conditions, err := s.listConditions(principal)
	if err != nil {
		return []model.App{}
	}
	apps, err := s.listApps(context.Background(), conditions...)
	if err != nil {
		return []model.App{}
	}
	items := make([]model.App, 0, len(apps))
	for i := range apps {
		if s.accessible(&apps[i], principal) == nil {
			items = append(items, apps[i])
		}
	}
	return items
}

func (s *AppService) Get(_ context.Context, id string) (model.App, error) {
	app, err := s.getByKey(context.Background(), id)
	if err != nil {
		return model.App{}, notFound("app %s not found", id)
	}
	return *app, nil
}

func (s *AppService) GetAuthorized(_ context.Context, principal iamprincipal.Principal, id string) (model.App, error) {
	app, err := s.Get(context.Background(), id)
	if err != nil {
		return model.App{}, err
	}
	if err := s.accessible(&app, principal); err != nil {
		return model.App{}, err
	}
	return app, nil
}

func (s *AppService) Heartbeat(_ context.Context, principal iamprincipal.Principal, req gatewaydto.HeartbeatRequest) (gatewaydto.App, error) {
	if req.AppID == "" {
		return gatewaydto.App{}, badRequest("appId is required")
	}
	now := time.Now().UTC()
	app, err := s.getByKey(context.Background(), req.AppID)
	if err != nil {
		return gatewaydto.App{}, notFound("app %s not found", req.AppID)
	}
	if err := s.accessibleBusiness(app, principal); err != nil {
		return gatewaydto.App{}, err
	}
	ttl := app.LeaseTTLSeconds
	if req.TTLSeconds > 0 {
		ttl = req.TTLSeconds
		app.LeaseTTLSeconds = ttl
	}
	if req.Version != "" {
		app.Version = req.Version
	}
	if len(req.Capabilities) > 0 {
		app.Capabilities = cloneStrings(req.Capabilities)
	}
	app.LastHeartbeatAt = &now
	leaseExpiresAt := now.Add(time.Duration(ttl) * time.Second)
	app.LeaseExpiresAt = &leaseExpiresAt
	app.UpdateAt = now
	if err := s.accessor.Save(context.Background(), app); err != nil {
		return gatewaydto.App{}, err
	}
	result := *app
	s.realtime.Publish("app.heartbeat", app.AppKey, appToDTO(&result))
	return appToDTO(&result), nil
}

func (s *AppService) MarkConnected(appID string) {
	app, err := s.getByKey(context.Background(), appID)
	if err != nil || app.Status != "active" {
		return
	}
	now := time.Now().UTC()
	leaseExpiresAt := now.Add(time.Duration(app.LeaseTTLSeconds) * time.Second)
	app.LastHeartbeatAt = &now
	app.LeaseExpiresAt = &leaseExpiresAt
	app.UpdateAt = now
	if err := s.accessor.Save(context.Background(), app); err != nil {
		return
	}
	result := *app
	s.realtime.Publish("app.heartbeat", app.AppKey, appToDTO(&result))
}

func (s *AppService) SetStatus(_ context.Context, principal iamprincipal.Principal, id string, enabled bool) (model.App, error) {
	now := time.Now().UTC()
	status := "disabled"
	if enabled {
		status = "active"
	}
	app, err := s.getByKey(context.Background(), id)
	if err != nil {
		return model.App{}, notFound("app %s not found", id)
	}
	if err := s.accessible(app, principal); err != nil {
		return model.App{}, err
	}
	app.Status = status
	app.UpdateAt = now
	if err := s.accessor.Save(context.Background(), app); err != nil {
		return model.App{}, err
	}
	result := *app
	s.realtime.Publish("app.status_changed", id, appToDTO(&result))
	return result, nil
}

func (s *AppService) accessible(app *model.App, principal iamprincipal.Principal) error {
	if principal.HasKind(iampkg.PrincipalKindHuman) {
		if !principal.HasScope(iampkg.ScopeGatewayAdmin) {
			return forbidden("principal cannot manage business apps")
		}
		if app.TenantID == "" || app.TenantID != principal.TenantID {
			return forbidden("app %s does not belong to staff tenant", app.AppKey)
		}
		return nil
	}
	if !principal.HasKind(iampkg.PrincipalKindBusiness) {
		return forbidden("relay principal cannot access business channel")
	}
	if !principal.HasScope(iampkg.ScopeChannelAccess) || principal.AccountKey != app.AppKey {
		return forbidden("channel credential cannot access app %s", app.AppKey)
	}
	if app.TenantID == "" || app.TenantID != principal.TenantID {
		return forbidden("app %s does not belong to account tenant", app.AppKey)
	}
	return nil
}

func (s *AppService) accessibleBusiness(app *model.App, principal iamprincipal.Principal) error {
	if !principal.HasKind(iampkg.PrincipalKindBusiness) {
		return forbidden("only business credentials can access business channel")
	}
	if !principal.HasScope(iampkg.ScopeChannelAccess) || principal.AccountKey != app.AppKey {
		return forbidden("channel credential cannot access app %s", app.AppKey)
	}
	if app.TenantID == "" || app.TenantID != principal.TenantID {
		return forbidden("app %s does not belong to account tenant", app.AppKey)
	}
	return nil
}

func (s *AppService) AuthorizeAccount(appID string, principal iamprincipal.Principal) error {
	if appID == "" {
		return badRequest("appId is required")
	}
	app, err := s.getByKey(context.Background(), appID)
	if err != nil {
		return notFound("app %s not found", appID)
	}
	if app.Status != "active" {
		return forbidden("app %s is disabled", appID)
	}
	if err := s.accessibleBusiness(app, principal); err != nil {
		return err
	}
	return nil
}

func (s *AppService) listApps(ctx context.Context, conditions ...dataaccess.QueryCondition) ([]model.App, error) {
	conditions = append(conditions, dataaccess.Order("create_at", dataaccess.OrderTypeAsc))
	var found []*model.App
	if err := s.accessor.ListWithOption(ctx, &found, dataaccess.WithConditions(conditions...)); err != nil {
		return nil, err
	}
	apps := make([]model.App, 0, len(found))
	for _, item := range found {
		apps = append(apps, *item)
	}
	return apps, nil
}

func (s *AppService) listConditions(principal iamprincipal.Principal) ([]dataaccess.QueryCondition, error) {
	if principal.TenantID == "" {
		return []dataaccess.QueryCondition{}, nil
	}
	switch {
	case principal.HasKind(iampkg.PrincipalKindHuman) && principal.HasScope(iampkg.ScopeGatewayAdmin):
		return []dataaccess.QueryCondition{dataaccess.Eq("tenant_id", principal.TenantID)}, nil
	case principal.HasKind(iampkg.PrincipalKindBusiness) && principal.HasScope(iampkg.ScopeChannelAccess):
		return []dataaccess.QueryCondition{
			dataaccess.Eq("tenant_id", principal.TenantID),
			dataaccess.Eq("app_key", principal.AccountKey),
		}, nil
	default:
		return []dataaccess.QueryCondition{}, nil
	}
}

func (s *AppService) getByKey(ctx context.Context, appID string) (*model.App, error) {
	conditions := []dataaccess.QueryCondition{
		dataaccess.Eq("app_key", appID),
		dataaccess.Limit(1),
	}
	var found []*model.App
	if err := s.accessor.ListWithOption(ctx, &found, dataaccess.WithConditions(conditions...)); err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, dataaccess.ErrDataNotFound
	}
	return found[0], nil
}

func (s *AppService) GetByTenantAndService(ctx context.Context, tenantID, serviceKey string) (*model.App, error) {
	conditions := []dataaccess.QueryCondition{
		dataaccess.Eq("tenant_id", tenantID),
		dataaccess.Eq("service_key", serviceKey),
		dataaccess.Limit(1),
	}
	var found []*model.App
	if err := s.accessor.ListWithOption(ctx, &found, dataaccess.WithConditions(conditions...)); err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, nil
	}
	return found[0], nil
}
