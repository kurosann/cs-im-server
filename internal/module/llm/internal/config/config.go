package config

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	DefaultPath          = "config/llm.yaml"
	DefaultRequestBytes  = 1 << 20
	DefaultResponseBytes = 16 << 20
	DefaultTimeout       = 60 * time.Second
)

var supportedProviders = map[string]string{
	"openai": "openai", "openai-compatible": "openai", "volcengine": "openai",
	"volc": "openai", "minimax": "openai", "aliyun": "openai", "alibaba": "openai", "alibaba-cloud": "openai", "qwen": "openai",
	"opencode": "openai", "anthropic": "anthropic",
}

type Config struct {
	Enabled                 bool                   `yaml:"enabled"`
	DefaultTranslationModel string                 `yaml:"default_translation_model"`
	MaxRequestBytes         int64                  `yaml:"max_request_bytes"`
	MaxResponseBytes        int64                  `yaml:"max_response_bytes"`
	Models                  map[string]ModelConfig `yaml:"models"`
}

// ModelConfig stores only a secret environment variable name, never a key value.
type ModelConfig struct {
	Disabled              bool          `yaml:"disabled"`
	Provider              string        `yaml:"provider"`
	Endpoint              string        `yaml:"endpoint"`
	Model                 string        `yaml:"model"`
	APIKeyEnv             string        `yaml:"api_key_env"`
	Timeout               time.Duration `yaml:"-"`
	TimeoutSeconds        int           `yaml:"timeout_seconds"`
	MaxTokens             int           `yaml:"max_tokens"`
	Temperature           *float64      `yaml:"temperature"`
	AnthropicVersion      string        `yaml:"anthropic_version"`
	OpenCodeSessionHeader bool          `yaml:"opencode_session_header"`
	resolvedAPIKey        string
}

func Secret(model ModelConfig) string { return model.resolvedAPIKey }

func Load(path string) (Config, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		path = DefaultPath
	}
	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("open llm config %s: %w", path, err)
	}
	defer file.Close()
	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	var cfg Config
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode llm config %s: %w", path, err)
	}
	var extra any
	if err := decoder.Decode(&extra); err == nil {
		return Config{}, errors.New("llm config must contain one YAML document")
	} else if !errors.Is(err, io.EOF) {
		return Config{}, fmt.Errorf("decode trailing llm config data: %w", err)
	}
	if cfg.MaxRequestBytes == 0 {
		cfg.MaxRequestBytes = DefaultRequestBytes
	}
	if cfg.MaxResponseBytes == 0 {
		cfg.MaxResponseBytes = DefaultResponseBytes
	}
	if cfg.MaxRequestBytes < 1 || cfg.MaxRequestBytes > 64<<20 {
		return Config{}, errors.New("max_request_bytes is outside the allowed range")
	}
	if cfg.MaxResponseBytes < 1 || cfg.MaxResponseBytes > 128<<20 {
		return Config{}, errors.New("max_response_bytes is outside the allowed range")
	}
	if len(cfg.Models) > 256 {
		return Config{}, errors.New("models cannot contain more than 256 entries")
	}
	cfg.DefaultTranslationModel = strings.TrimSpace(cfg.DefaultTranslationModel)
	models := make(map[string]ModelConfig, len(cfg.Models))
	for rawSelector, model := range cfg.Models {
		selector := strings.TrimSpace(rawSelector)
		if selector == "" || strings.ContainsAny(selector, " /\\") {
			return Config{}, fmt.Errorf("invalid model selector %q", selector)
		}
		if _, exists := models[selector]; exists {
			return Config{}, fmt.Errorf("duplicate model selector %q", selector)
		}
		provider := strings.ToLower(strings.TrimSpace(model.Provider))
		protocol, ok := supportedProviders[provider]
		if !ok {
			return Config{}, fmt.Errorf("model %q uses unsupported provider %q", selector, provider)
		}
		model.Provider = provider
		model.Endpoint = strings.TrimSpace(model.Endpoint)
		model.Model = strings.TrimSpace(model.Model)
		model.APIKeyEnv = strings.TrimSpace(model.APIKeyEnv)
		if model.Model == "" || model.Endpoint == "" {
			return Config{}, fmt.Errorf("model %q requires endpoint and model", selector)
		}
		parsed, err := url.Parse(model.Endpoint)
		if err != nil || !parsed.IsAbs() || parsed.Host == "" || parsed.User != nil || parsed.Scheme != "https" || parsed.RawQuery != "" || parsed.Fragment != "" {
			return Config{}, fmt.Errorf("model %q requires an absolute HTTPS endpoint without user info", selector)
		}
		if !model.Disabled && model.APIKeyEnv == "" {
			return Config{}, fmt.Errorf("model %q requires api_key_env", selector)
		}
		if !model.Disabled && strings.TrimSpace(os.Getenv(model.APIKeyEnv)) == "" {
			return Config{}, fmt.Errorf("environment variable %s for model %q is empty", model.APIKeyEnv, selector)
		}
		if model.APIKeyEnv != "" {
			model.resolvedAPIKey = strings.TrimSpace(os.Getenv(model.APIKeyEnv))
		}
		if model.TimeoutSeconds == 0 {
			model.Timeout = DefaultTimeout
		} else if model.TimeoutSeconds < 1 || model.TimeoutSeconds > 600 {
			return Config{}, fmt.Errorf("model %q timeout_seconds must be between 1 and 600", selector)
		} else {
			model.Timeout = time.Duration(model.TimeoutSeconds) * time.Second
		}
		if model.MaxTokens < 0 {
			return Config{}, fmt.Errorf("model %q max_tokens cannot be negative", selector)
		}
		if model.Temperature != nil && (*model.Temperature < 0 || *model.Temperature > 2) {
			return Config{}, fmt.Errorf("model %q temperature must be between 0 and 2", selector)
		}
		if model.OpenCodeSessionHeader && !strings.EqualFold(parsed.Hostname(), "opencode.ai") {
			return Config{}, fmt.Errorf("model %q enables OpenCode session header for a non-OpenCode host", selector)
		}
		if protocol == "anthropic" {
			if model.AnthropicVersion == "" {
				model.AnthropicVersion = "2023-06-01"
			}
			if model.MaxTokens == 0 {
				model.MaxTokens = 4096
			}
		}
		if model.resolvedAPIKey == "" {
			model.resolvedAPIKey = strings.TrimSpace(os.Getenv(model.APIKeyEnv))
		}
		models[selector] = model
	}
	cfg.Models = models
	if cfg.DefaultTranslationModel != "" {
		model, ok := cfg.Models[cfg.DefaultTranslationModel]
		if !ok || model.Disabled {
			return Config{}, errors.New("default_translation_model must name an enabled model")
		}
	}
	if cfg.Enabled && len(cfg.Selectors()) == 0 {
		return Config{}, errors.New("enabled llm config requires at least one enabled model")
	}
	return cfg, nil
}

func Protocol(provider string) string {
	return supportedProviders[strings.ToLower(strings.TrimSpace(provider))]
}

func (c Config) Selectors() []string {
	selectors := make([]string, 0, len(c.Models))
	for selector, model := range c.Models {
		if c.Enabled && !model.Disabled {
			selectors = append(selectors, selector)
		}
	}
	sort.Strings(selectors)
	return selectors
}
