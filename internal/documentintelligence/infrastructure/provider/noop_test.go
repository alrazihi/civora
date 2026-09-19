package provider

import (
	"context"
	"testing"

	"github.com/alrazihi/civora/internal/documentintelligence/application"
	"github.com/alrazihi/civora/internal/documentintelligence/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestNoopProvider_GenerateDocumentAnalyses(t *testing.T) {
	provider := NewNoopProvider()

	req := application.ProviderRequest{
		OrganizationID: uuid.New(),
		EvidenceID:     uuid.New(),
		Documents: []domain.DocumentContent{
			{
				DocumentID:  uuid.New(),
				FileName:    "test.pdf",
				ContentType: "application/pdf",
				Content:     []byte("test content"),
				Checksum:    "abc123",
			},
		},
		Options: domain.ProviderOptions{},
	}

	_, err := provider.GenerateDocumentAnalyses(context.Background(), req)

	assert.Error(t, err)
	assert.Equal(t, ErrProviderDisabled, err)
}
