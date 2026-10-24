package ai

import (
	"context"

	aidto "cs-im-server/internal/module/ai/dto"
	service "cs-im-server/internal/module/ai/internal/service"
)

type AIService interface {
	Generate(ctx context.Context, req GenerateRequest) (GenerateResult, error)
	HasActiveProvider(ctx context.Context) bool
}

func SetService(next AIService) {
	service.SetGlobalService(next)
}

func Service() AIService {
	return service.GlobalService()
}

type GenerateRequest = aidto.GenerateRequest
type GenerateResult = aidto.GenerateResult
