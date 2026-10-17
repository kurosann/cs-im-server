package realtime

import "sync"

import (
	commonmodule "vane-common/module"

	realtimehttp "cs-im-server/internal/module/gateway/internal/realtime/http"
	service "cs-im-server/internal/module/gateway/internal/realtime/internal/service"
)

type (
	RealtimeEvent   = service.RealtimeEvent
	RealtimeService = service.RealtimeService
)

var serviceMu sync.RWMutex
var realtimeService *RealtimeService

func SetService(next *RealtimeService) {
	serviceMu.Lock()
	defer serviceMu.Unlock()
	realtimeService = next
}

func Service() *RealtimeService {
	serviceMu.RLock()
	defer serviceMu.RUnlock()
	if realtimeService == nil {
		panic("realtime service is not initialized")
	}
	return realtimeService
}

func Init() { SetService(service.NewRealtimeService()) }

func NewRealtimeService() *RealtimeService {
	return service.NewRealtimeService()
}

func Routes() commonmodule.HttpRouteGroup { return realtimehttp.NewRealtimeAPI(Service()).Routes() }
