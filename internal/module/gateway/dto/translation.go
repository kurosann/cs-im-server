package dto

type TranslateRequest struct {
	Text       string `json:"text"`
	SourceLang string `json:"sourceLang"`
	TargetLang string `json:"targetLang"`
}

type TranslateResponse struct {
	Text     string `json:"text"`
	Provider string `json:"provider"`
}
