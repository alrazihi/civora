package provider

import (
	"github.com/alrazihi/civora/internal/documentintelligence/application"
)

type LocalProvider struct {
	*OpenAIProvider
}

func NewLocalProvider(cfg OpenAIProviderConfig) *LocalProvider {
	cfg.SkipAuth = true
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://localhost:11434/v1"
	}
	if cfg.Model == "" {
		cfg.Model = "llama3"
	}
	return &LocalProvider{OpenAIProvider: NewOpenAIProvider(cfg)}
}

var _ application.AIProvider = (*LocalProvider)(nil)
