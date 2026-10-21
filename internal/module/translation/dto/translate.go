package dto

type TranslateItem struct {
	Text       string `json:"text"`
	TargetLang string `json:"targetLang"`
	SourceLang string `json:"sourceLang"`
	ProviderID string `json:"providerId"`
}

type TranslateRequest struct {
	Items  []TranslateItem `json:"items"`
	Stream bool            `json:"stream"`
}

type TranslateResult struct {
	Text           string `json:"text"`
	TargetLang     string `json:"targetLang"`
	SourceLang     string `json:"sourceLang"`
	TranslatedText string `json:"translatedText"`
	Cached         bool   `json:"cached"`
	QualityTier    string `json:"qualityTier"`
	ProviderID     string `json:"-"`
	ProviderKind   string `json:"-"`
	ModelID        string `json:"-"`
	Priority       int    `json:"priority"`
	TextHash       string `json:"-"`
	ResultKey      string `json:"resultKey"`
}
