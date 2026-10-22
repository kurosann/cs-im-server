// Package llm exposes the in-process interface used by other server modules.
package llm

import (
	"context"
	"strings"
	"sync"
)

type Client interface {
	TranslationSelector() string
	TranslateText(ctx context.Context, text string, sourceLang string, targetLang string) (string, error)
}

var (
	mu         sync.RWMutex
	registered Client
)

func Register(client Client) {
	mu.Lock()
	defer mu.Unlock()
	registered = client
}

func Get() Client {
	mu.RLock()
	defer mu.RUnlock()
	if registered == nil {
		panic("llm module is not initialized")
	}
	return registered
}

func TryGet() (Client, bool) {
	mu.RLock()
	defer mu.RUnlock()
	return registered, registered != nil
}

func SessionID(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 128 {
		value = value[:128]
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '-' || char == '_' || char == '.' {
			continue
		}
		return ""
	}
	return value
}
