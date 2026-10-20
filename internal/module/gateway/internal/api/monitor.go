package api

import (
	nethttp "net/http"
	"time"

	"cs-im-server/internal/module/gateway/internal/service"
	iampkg "cs-im-server/internal/module/iam/pkg/iam"
	relaypkg "cs-im-server/internal/module/relay/pkg"
	"cs-im-server/pkg/httpapi"
	commonmodule "vane-common/module"
)

type MonitorAPI struct {
	Apps          *service.AppService
	Conversations *service.ConversationService
}

func NewMonitorAPI(apps *service.AppService, conversations *service.ConversationService) *MonitorAPI {
	return &MonitorAPI{Apps: apps, Conversations: conversations}
}

type MonitorChannel struct {
	ChannelID    string                `json:"channelId"`
	Name         string                `json:"name"`
	Type         string                `json:"type"`
	Endpoint     string                `json:"endpoint,omitempty"`
	Status       string                `json:"status"`
	Health       string                `json:"health"`
	LastTestedAt *time.Time            `json:"lastTestedAt,omitempty"`
	Delivery     service.DeliveryStats `json:"delivery"`
}

type MonitorApp struct {
	AppID             string     `json:"appId"`
	DisplayName       string     `json:"displayName"`
	ServiceKey        string     `json:"serviceKey"`
	Kind              string     `json:"kind,omitempty"`
	Version           string     `json:"version,omitempty"`
	Status            string     `json:"status"`
	Health            string     `json:"health"`
	LastHeartbeatAt   *time.Time `json:"lastHeartbeatAt,omitempty"`
	LeaseExpiresAt    *time.Time `json:"leaseExpiresAt,omitempty"`
	ConversationCount int        `json:"conversationCount"`
}

type MonitorSummary struct {
	TotalChannels int `json:"totalChannels"`
	Online        int `json:"online"`
	Offline       int `json:"offline"`
	Disabled      int `json:"disabled"`
	QueuedItems   int `json:"queuedItems"`
}

type MonitorResult struct {
	GeneratedAt time.Time        `json:"generatedAt"`
	Summary     MonitorSummary   `json:"summary"`
	Channels    []MonitorChannel `json:"channels"`
	Apps        []MonitorApp     `json:"apps"`
}

func (a *MonitorAPI) Admin(ctx httpapi.Context) {
	principal, ok := iampkg.PrincipalFromContext(ctx.Context())
	if !ok {
		writeServiceError(ctx, iampkg.ErrPrincipalMissing)
		return
	}

	now := time.Now().UTC()
	apps := a.Apps.List(ctx.Context(), principal)
	existingAppIDs, err := iampkg.Service().ExistingChannelAppIDs(ctx.Context(), principal)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	deliveryStats := a.Conversations.DeliveryStatsByChannel()
	conversationCounts := a.Conversations.ConversationCountByApp()

	monitorApps := make([]MonitorApp, 0, len(apps))
	for _, app := range apps {
		if _, exists := existingAppIDs[app.AppKey]; !exists {
			continue
		}
		health := "online"
		switch {
		case app.Status == "disabled":
			health = "disabled"
		case app.LeaseExpiresAt != nil && app.LeaseExpiresAt.Before(now):
			health = "offline"
		case app.LastHeartbeatAt == nil:
			health = "offline"
		}
		monitorApps = append(monitorApps, MonitorApp{
			AppID:             app.AppKey,
			DisplayName:       app.DisplayName,
			ServiceKey:        app.ServiceKey,
			Kind:              app.Kind,
			Version:           app.Version,
			Status:            app.Status,
			Health:            health,
			LastHeartbeatAt:   app.LastHeartbeatAt,
			LeaseExpiresAt:    app.LeaseExpiresAt,
			ConversationCount: conversationCounts[app.AppKey],
		})
	}

	channels := relaypkg.Service().ListChannels(ctx.Context())
	monitorChannels := make([]MonitorChannel, 0, len(channels))
	for _, channel := range channels {
		stats := deliveryStats[channel.ChannelKey]
		health := "online"
		if channel.Status != "active" {
			health = "disabled"
		} else if stats.Queued > 0 {
			health = "warning"
		}
		monitorChannels = append(monitorChannels, MonitorChannel{
			ChannelID:    channel.ChannelKey,
			Name:         channel.Name,
			Type:         channel.Type,
			Endpoint:     channel.Endpoint,
			Status:       channel.Status,
			Health:       health,
			LastTestedAt: channel.LastTestedAt,
			Delivery:     stats,
		})
	}

	summary := MonitorSummary{TotalChannels: len(monitorChannels)}
	for _, channel := range monitorChannels {
		summary.QueuedItems += channel.Delivery.Queued
		switch channel.Health {
		case "online":
			summary.Online++
		case "offline":
			summary.Offline++
		case "disabled":
			summary.Disabled++
		}
	}

	httpapi.WriteJSON(ctx, nethttp.StatusOK, "OK", MonitorResult{
		GeneratedAt: now,
		Summary:     summary,
		Channels:    monitorChannels,
		Apps:        monitorApps,
	})
}

func (a *MonitorAPI) AdminRoutes() commonmodule.HttpRouteGroup {
	routes := commonmodule.NewHttpRouteGroup("/channel-monitor")
	routes.Get("", a.Admin)
	return routes
}
