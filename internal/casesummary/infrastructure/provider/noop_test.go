package provider

import (
	"context"
	"errors"
	"testing"

	"github.com/alrazihi/civora/internal/casesummary/application"
)

func TestNoopProvider_GenerateCaseSummary(t *testing.T) {
	p := NewNoopProvider()

	_, err := p.GenerateCaseSummary(context.Background(), application.CaseSummaryRequest{})
	if err == nil {
		t.Fatal("expected error from noop provider")
	}
	if !errors.Is(err, ErrProviderDisabled) {
		t.Errorf("expected ErrProviderDisabled, got %v", err)
	}
}

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
