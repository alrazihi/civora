package provider

import (
	"context"
	"errors"

	"github.com/alrazihi/civora/internal/documentintelligence/application"
	"github.com/alrazihi/civora/internal/documentintelligence/domain"
)

var ErrProviderDisabled = errors.New("document intelligence provider is not configured or is disabled")

type NoopProvider struct{}

func NewNoopProvider() *NoopProvider {
	return &NoopProvider{}
}

func (p *NoopProvider) ProviderInfo() domain.ModelInfo {
	return domain.ModelInfo{
		Name:     "noop",
		Version:  "0.0",
		Provider: "none",
	}
}

func (p *NoopProvider) GenerateDocumentAnalyses(ctx context.Context, req application.ProviderRequest) ([]application.AnalysisResult, error) {
	return nil, ErrProviderDisabled
}

var _ application.AIProvider = (*NoopProvider)(nil)
