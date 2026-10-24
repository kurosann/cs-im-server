package service

import (
	"context"
	"sync"

	aidto "cs-im-server/internal/module/ai/dto"
)

type AIService interface {
	Generate(ctx context.Context, req aidto.GenerateRequest) (aidto.GenerateResult, error)
	HasActiveProvider(ctx context.Context) bool
}

var (
	globalMu        sync.RWMutex
	globalAIService AIService
)

func SetGlobalService(next AIService) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalAIService = next
}

func GlobalService() AIService {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalAIService
}
