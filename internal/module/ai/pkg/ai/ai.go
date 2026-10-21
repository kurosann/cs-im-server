package ai

import (
	"context"
	"sync"

	aidto "cs-im-server/internal/module/ai/dto"
)

var serviceMu sync.RWMutex
var providerService AIService

type AIService interface {
	Generate(ctx context.Context, req GenerateRequest) (GenerateResult, error)
	HasActiveProvider(ctx context.Context) bool
}

func SetService(next AIService) {
	serviceMu.Lock()
	defer serviceMu.Unlock()
	providerService = next
}

func Service() AIService {
	serviceMu.RLock()
	defer serviceMu.RUnlock()
	if providerService == nil {
		return nil
	}
	return providerService
}

type GenerateRequest = aidto.GenerateRequest
type GenerateResult = aidto.GenerateResult
