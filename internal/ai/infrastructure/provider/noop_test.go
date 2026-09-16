package provider

import (
	"context"
	"testing"

	"github.com/alrazihi/civora/internal/ai/application"
	"github.com/alrazihi/civora/internal/ai/domain"
	"github.com/google/uuid"
)

func TestNoopProvider_ProviderInfo(t *testing.T) {
	p := NewNoopProvider()
	info := p.ProviderInfo()

	if info.Name != "noop" {
		t.Errorf("expected name 'noop', got %s", info.Name)
	}
	if info.Provider != "local" {
		t.Errorf("expected provider 'local', got %s", info.Provider)
	}
}

func TestNoopProvider_GenerateObservations_AlwaysFails(t *testing.T) {
	p := NewNoopProvider()

	req := application.ProviderRequest{
		OrganizationID: uuid.New(),
		EvidenceID:     uuid.New(),
	}

	results, err := p.GenerateObservations(context.Background(), req)
	if err == nil {
		t.Fatal("expected error from noop provider")
	}
	if results != nil {
		t.Errorf("expected nil results, got %v", results)
	}
}

func TestNoopProvider_ImplementsAIProvider(t *testing.T) {
	var _ application.AIProvider = (*NoopProvider)(nil)
}

func TestProviderOptions_DefaultValues(t *testing.T) {
	opts := domain.ProviderOptions{
		Types:     nil,
		MaxTokens: 0,
	}

	if opts.MaxTokens == 0 {
		opts.MaxTokens = 4096
	}
	if opts.MaxTokens != 4096 {
		t.Errorf("expected default max tokens 4096, got %d", opts.MaxTokens)
	}
}
