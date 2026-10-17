package service

import (
	"context"
	"sort"
	"sync"
	"time"

	"vane-common/pkg/dataaccess"
	"vane-common/pkg/idutil"

	iampkg "cs-im-server/internal/module/iam/pkg"
)

type AppService struct {
	mu           sync.RWMutex
	accessor     dataaccess.DataAccessor[*App]
	apps         map[string]*App
	idsByService map[string]string
	realtime     *RealtimeService
}

type RegisterAppRequest struct {
	ServiceKey      string   `json:"serviceKey"`
	DisplayName     string   `json:"displayName"`
	Kind            string   `json:"kind"`
	Version         string   `json:"version"`
	Capabilities    []string `json:"capabilities"`
	LeaseTTLSeconds int      `json:"leaseTtlSeconds"`
}

type HeartbeatRequest struct {
	AppID        string   `json:"appId"`
	InstanceID   string   `json:"instanceId"`
	Version      string   `json:"version"`
	Capabilities []string `json:"capabilities"`
	TTLSeconds   int      `json:"ttlSeconds"`
}

func NewAppService(realtime *RealtimeService) *AppService {
	return &AppService{
		apps:         make(map[string]*App),
		idsByService: make(map[string]string),
		realtime:     realtime,
	}
}

func (s *AppService) Register(_ context.Context, principal Principal, req RegisterAppRequest) (App, error) {
	if req.ServiceKey == "" {
		return App{}, badRequest("serviceKey is required")
	}
	if req.DisplayName == "" {
		return App{}, badRequest("displayName is required")
	}

	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()

	if appID, ok := s.idsByService[req.ServiceKey]; ok {
		app := s.apps[appID]
		app.DisplayName = req.DisplayName
		app.Kind = req.Kind
		app.Version = req.Version
		app.Capabilities = cloneStrings(req.Capabilities)
		app.UpdateAt = now
		if err := s.accessor.Save(context.Background(), app); err != nil {
			return App{}, err
		}
		return *app, nil
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
		return App{}, err
	}
	s.apps[app.AppKey] = app
	s.idsByService[req.ServiceKey] = app.AppKey
	result := *app
	s.realtime.Publish("app.registered", id, result)
	return result, nil
}

func (s *AppService) List(_ context.Context) []App {
	s.mu.RLock()
	defer s.mu.RUnlock()
	apps := make([]App, 0, len(s.apps))
	for _, app := range s.apps {
		apps = append(apps, *app)
	}
	sort.Slice(apps, func(i, j int) bool { return apps[i].CreateAt.Before(apps[j].CreateAt) })
	return apps
}

func (s *AppService) Get(_ context.Context, id string) (App, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	app, ok := s.apps[id]
	if !ok {
		return App{}, notFound("app %s not found", id)
	}
	return *app, nil
}

func (s *AppService) Heartbeat(_ context.Context, req HeartbeatRequest) (App, error) {
	if req.AppID == "" {
		return App{}, badRequest("appId is required")
	}
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	app, ok := s.apps[req.AppID]
	if !ok {
		return App{}, notFound("app %s not found", req.AppID)
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
		return App{}, err
	}
	result := *app
	s.realtime.Publish("app.heartbeat", app.AppKey, result)
	return result, nil
}

func (s *AppService) SetStatus(_ context.Context, id string, enabled bool) (App, error) {
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
	app.Status = status
	app.UpdateAt = now
	if err := s.accessor.Save(context.Background(), app); err != nil {
		return App{}, err
	}
	result := *app
	s.realtime.Publish("app.status_changed", id, result)
	return result, nil
}

func (s *AppService) AuthorizeAccount(appID string, principal Principal) error {
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
	if app.TenantID == "" || app.TenantID != principal.TenantID {
		return forbidden("app %s does not belong to account tenant", appID)
	}
	if !principal.HasKind(iampkg.PrincipalKindBusiness) {
		return forbidden("relay principal cannot access business channel")
	}
	return nil
}

func (s *AppService) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	items, err := s.accessor.List(context.Background())
	if err != nil {
		return err
	}
	for _, item := range items {
		s.apps[item.AppKey] = item
		s.idsByService[item.ServiceKey] = item.AppKey
	}
	return nil
}
