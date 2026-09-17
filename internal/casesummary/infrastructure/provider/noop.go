package provider

import (
	"context"

	"github.com/alrazihi/civora/internal/casesummary/application"
	"github.com/alrazihi/civora/internal/casesummary/domain"
)

type NoopProvider struct{}

func NewNoopProvider() *NoopProvider {
	return &NoopProvider{}
}

func (p *NoopProvider) GenerateCaseSummary(ctx context.Context, req application.CaseSummaryRequest) (*application.CaseSummaryResult, error) {
	return nil, ErrProviderDisabled
}

func (p *NoopProvider) ProviderInfo() domain.ModelInfo {
	return domain.ModelInfo{
		Name:     "noop",
		Version:  "0.0.0",
		Provider: "local",
	}
}

var _ application.CaseSummaryProvider = (*NoopProvider)(nil)
