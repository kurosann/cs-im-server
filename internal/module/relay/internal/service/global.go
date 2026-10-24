package service

import "sync"

var (
	globalMu       sync.RWMutex
	globalRelaySvc *RelayService
)

func SetGlobalService(next *RelayService) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalRelaySvc = next
}

func GlobalService() *RelayService {
	globalMu.RLock()
	defer globalMu.RUnlock()
	if globalRelaySvc == nil {
		panic("relay service is not initialized")
	}
	return globalRelaySvc
}
