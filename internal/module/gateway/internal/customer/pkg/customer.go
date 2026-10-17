package customer

import "sync"

import (
	commonmodule "vane-common/module"

	customerhttp "cs-im-server/internal/module/gateway/internal/customer/http"
	service "cs-im-server/internal/module/gateway/internal/customer/internal/service"
	model "cs-im-server/internal/module/gateway/internal/customer/model"
	realtimepkg "cs-im-server/internal/module/gateway/internal/realtime/pkg"
)

type (
	Customer                      = model.Customer
	CustomerIdentity              = model.CustomerIdentity
	CustomerService               = service.CustomerService
	CreateCustomerRequest         = service.CreateCustomerRequest
	AddIdentityRequest            = service.AddIdentityRequest
	MergeCustomerRequest          = service.MergeCustomerRequest
	EnsureExternalCustomerRequest = service.EnsureExternalCustomerRequest
)

var serviceMu sync.RWMutex
var customerService *CustomerService

func SetService(next *CustomerService) {
	serviceMu.Lock()
	defer serviceMu.Unlock()
	customerService = next
}

func Service() *CustomerService {
	serviceMu.RLock()
	defer serviceMu.RUnlock()
	if customerService == nil {
		panic("customer service is not initialized")
	}
	return customerService
}

func Init(realtime *realtimepkg.RealtimeService) error {
	next := service.NewCustomerService(realtime)
	if err := next.Load(); err != nil {
		return err
	}
	SetService(next)
	return nil
}

func NewCustomerService(realtime *realtimepkg.RealtimeService) *CustomerService {
	return service.NewCustomerService(realtime)
}

func Models() []commonmodule.ModelRegInfo {
	return []commonmodule.ModelRegInfo{commonmodule.NewModelRegInfo(&model.Customer{})}
}

func Routes() commonmodule.HttpRouteGroup { return customerhttp.NewCustomerAPI(Service()).Routes() }
