package service

import (
	"context"
	"sort"
	"sync"
	"time"

	"vane-common/pkg/dataaccess"
	"vane-common/pkg/idutil"

	gatewaydto "cs-im-server/internal/module/gateway/dto"
	iampkg "cs-im-server/internal/module/iam/pkg/iam"
)

type AppService struct {
	mu           sync.RWMutex
	loadOnce     sync.Once
	loadErr      error
	accessor     dataaccess.DataAccessor[*App]
	apps         map[string]*App
	idsByService map[string]string
	realtime     *RealtimeService
}

func NewAppService(realtime *RealtimeService) *AppService {
	return &AppService{
		apps:         make(map[string]*App),
		idsByService: make(map[string]string),
		realtime:     realtime,
	}
}

func (s *AppService) Register(_ context.Context, principal Principal, req gatewaydto.RegisterAppRequest) (gatewaydto.App, error) {
	if err := s.ensureLoaded(); err != nil {
		return gatewaydto.App{}, err
	}
	if req.ServiceKey == "" {
		return gatewaydto.App{}, badRequest("serviceKey is required")
	}
	if req.DisplayName == "" {
		return gatewaydto.App{}, badRequest("displayName is required")
	}

	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()

	if appID, ok := s.idsByService[serviceKeyLookupKey(principal.TenantID, req.ServiceKey)]; ok {
		app := s.apps[appID]
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
	app := &App{
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
	s.apps[app.AppKey] = app
	s.idsByService[serviceKeyLookupKey(app.TenantID, req.ServiceKey)] = app.AppKey
	result := *app
	s.realtime.Publish("app.registered", id, appToDTO(&result))
	return appToDTO(&result), nil
}

func (s *AppService) List(_ context.Context, principal Principal) []App {
	if err := s.ensureLoaded(); err != nil {
		return []App{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	apps := make([]App, 0, len(s.apps))
	for _, app := range s.apps {
		if s.accessible(app, principal) != nil {
			continue
		}
		apps = append(apps, *app)
	}
	sort.Slice(apps, func(i, j int) bool { return apps[i].CreateAt.Before(apps[j].CreateAt) })
	return apps
}

func (s *AppService) Get(_ context.Context, id string) (App, error) {
	if err := s.ensureLoaded(); err != nil {
		return App{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	app, ok := s.apps[id]
	if !ok {
		return App{}, notFound("app %s not found", id)
	}
	return *app, nil
}

func (s *AppService) GetAuthorized(_ context.Context, principal Principal, id string) (App, error) {
	if err := s.ensureLoaded(); err != nil {
		return App{}, err
	}
	app, err := s.Get(context.Background(), id)
	if err != nil {
		return App{}, err
	}
	if err := s.accessible(&app, principal); err != nil {
		return App{}, err
	}
	return app, nil
}

func (s *AppService) Heartbeat(_ context.Context, principal Principal, req gatewaydto.HeartbeatRequest) (gatewaydto.App, error) {
	if err := s.ensureLoaded(); err != nil {
		return gatewaydto.App{}, err
	}
	if req.AppID == "" {
		return gatewaydto.App{}, badRequest("appId is required")
	}
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	app, ok := s.apps[req.AppID]
	if !ok {
		return gatewaydto.App{}, notFound("app %s not found", req.AppID)
	}
	if err := s.accessible(app, principal); err != nil {
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
	if err := s.ensureLoaded(); err != nil {
		return
	}
	s.mu.Lock()
	app, ok := s.apps[appID]
	if !ok || app.Status != "active" {
		s.mu.Unlock()
		return
	}
	now := time.Now().UTC()
	leaseExpiresAt := now.Add(time.Duration(app.LeaseTTLSeconds) * time.Second)
	app.LastHeartbeatAt = &now
	app.LeaseExpiresAt = &leaseExpiresAt
	app.UpdateAt = now
	if err := s.accessor.Save(context.Background(), app); err != nil {
		s.mu.Unlock()
		return
	}
	result := *app
	s.mu.Unlock()
	s.realtime.Publish("app.heartbeat", app.AppKey, appToDTO(&result))
}

func (s *AppService) SetStatus(_ context.Context, principal Principal, id string, enabled bool) (App, error) {
	if err := s.ensureLoaded(); err != nil {
		return App{}, err
	}
	now := time.Now().UTC()
	status := "disabled"
	if enabled {
		status = "active"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	app, ok := s.apps[id]
	if !ok {
		return App{}, notFound("app %s not found", id)
	}
	if err := s.accessible(app, principal); err != nil {
		return App{}, err
	}
	app.Status = status
	app.UpdateAt = now
	if err := s.accessor.Save(context.Background(), app); err != nil {
		return App{}, err
	}
	result := *app
	s.realtime.Publish("app.status_changed", id, appToDTO(&result))
	return result, nil
}

func (s *AppService) accessible(app *App, principal Principal) error {
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

func (s *AppService) AuthorizeAccount(appID string, principal Principal) error {
	if err := s.ensureLoaded(); err != nil {
		return err
	}
	if appID == "" {
		return badRequest("appId is required")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	app, ok := s.apps[appID]
	if !ok {
		return notFound("app %s not found", appID)
	}
	if app.Status != "active" {
		return forbidden("app %s is disabled", appID)
	}
	if err := s.accessible(app, principal); err != nil {
		return err
	}
	return nil
}

func (s *AppService) Load() error {
	return s.ensureLoaded()
}

func (s *AppService) ensureLoaded() error {
	s.loadOnce.Do(func() {
		s.loadErr = s.load()
	})
	return s.loadErr
}

func (s *AppService) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	items, err := s.accessor.List(context.Background())
	if err != nil {
		return err
	}
	for _, item := range items {
		s.apps[item.AppKey] = item
		s.idsByService[serviceKeyLookupKey(item.TenantID, item.ServiceKey)] = item.AppKey
	}
	return nil
}

func serviceKeyLookupKey(tenantID, serviceKey string) string {
	return tenantID + "\x00" + serviceKey
}
