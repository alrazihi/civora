package provider

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/alrazihi/civora/internal/ai/application"
	"github.com/alrazihi/civora/internal/ai/domain"
)

const defaultLocalTimeout = 120 * time.Second

type LocalProvider struct {
	client  *OpenAIProvider
	model   string
	baseURL string
}

func NewLocalProvider(baseURL, model string) *LocalProvider {
	if baseURL == "" {
		baseURL = "http://localhost:11434/v1"
	}
	if model == "" {
		model = "llama3"
	}

	inf := NewOpenAIProvider(OpenAIProviderConfig{
		BaseURL:    baseURL,
		Model:      model,
		SkipAuth:   true,
		HTTPClient: &http.Client{Timeout: defaultLocalTimeout},
	})

	return &LocalProvider{
		client:  inf,
		model:   model,
		baseURL: baseURL,
	}
}

func (p *LocalProvider) GenerateObservations(ctx context.Context, req application.ProviderRequest) ([]application.ObservationResult, error) {
	if err := p.validateConfig(); err != nil {
		return nil, err
	}
	results, err := p.client.GenerateObservations(ctx, req)
	if err != nil {
		return nil, err
	}
	for i := range results {
		results[i].Model = p.ProviderInfo()
		results[i].Model.Provider = "local"
	}
	return results, nil
}

func (p *LocalProvider) validateConfig() error {
	if p.baseURL == "" {
		return fmt.Errorf("%w: local LLM server URL is not configured", ErrProviderDisabled)
	}
	if p.model == "" {
		return fmt.Errorf("local model is not configured")
	}
	return nil
}

func (p *LocalProvider) ProviderInfo() domain.ModelInfo {
	return domain.ModelInfo{
		Name:     p.model,
		Version:  "1.0",
		Provider: "local",
	}
}

var _ application.AIProvider = (*LocalProvider)(nil)
