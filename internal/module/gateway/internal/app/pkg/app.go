package app

import "sync"

import (
	commonmodule "vane-common/module"

	apphttp "cs-im-server/internal/module/gateway/internal/app/http"
	service "cs-im-server/internal/module/gateway/internal/app/internal/service"
	model "cs-im-server/internal/module/gateway/internal/app/model"
	realtimepkg "cs-im-server/internal/module/gateway/internal/realtime/pkg"
)

type (
	App                = model.App
	AppService         = service.AppService
	RegisterAppRequest = service.RegisterAppRequest
	HeartbeatRequest   = service.HeartbeatRequest
	RealtimeService    = realtimepkg.RealtimeService
)

var serviceMu sync.RWMutex
var appService *AppService

func SetService(next *AppService) {
	serviceMu.Lock()
	defer serviceMu.Unlock()
	appService = next
}

func Service() *AppService {
	serviceMu.RLock()
	defer serviceMu.RUnlock()
	if appService == nil {
		panic("app service is not initialized")
	}
	return appService
}

func Init(realtime *RealtimeService) error {
	next := service.NewAppService(realtime)
	if err := next.Load(); err != nil {
		return err
	}
	SetService(next)
	return nil
}

func Models() []commonmodule.ModelRegInfo {
	return []commonmodule.ModelRegInfo{commonmodule.NewModelRegInfo(&model.App{})}
}

func RuntimeRoutes(messages apphttp.MessageHistoryService) commonmodule.HttpRouteGroup {
	return apphttp.NewAppAPI(Service(), messages).RuntimeRoutes()
}

func AdminRoutes(messages apphttp.MessageHistoryService) commonmodule.HttpRouteGroup {
	return apphttp.NewAppAPI(Service(), messages).AdminRoutes()
}
