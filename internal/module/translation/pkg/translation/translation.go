package translation

import (
	translationdto "cs-im-server/internal/module/translation/dto"
	service "cs-im-server/internal/module/translation/internal/service"
)

type TranslationService = service.TranslationService
type TranslateRequest = translationdto.TranslateRequest
type TranslateResult = translationdto.TranslateResult
type TranslateItem = translationdto.TranslateItem

var Service *TranslationService

func SetService(next *TranslationService) { Service = next }
