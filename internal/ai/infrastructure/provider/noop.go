package provider

import (
	"context"
	"errors"

	"github.com/alrazihi/civora/internal/ai/application"
	"github.com/alrazihi/civora/internal/ai/domain"
)

var ErrProviderDisabled = errors.New("AI provider is not configured or is disabled")

type NoopProvider struct{}

func NewNoopProvider() *NoopProvider {
	return &NoopProvider{}
}

func (p *NoopProvider) GenerateObservations(ctx context.Context, req application.ProviderRequest) ([]application.ObservationResult, error) {
	return nil, ErrProviderDisabled
}

func (p *NoopProvider) ProviderInfo() domain.ModelInfo {
	return domain.ModelInfo{
		Name:     "noop",
		Version:  "0.0.0",
		Provider: "local",
	}
}

var _ application.AIProvider = (*NoopProvider)(nil)
