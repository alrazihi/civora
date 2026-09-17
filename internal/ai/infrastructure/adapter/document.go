package adapter

import (
	"context"
	"fmt"
	"io"

	evidenceapp "github.com/alrazihi/civora/internal/evidence/application"
	"github.com/google/uuid"
)

// EvidenceDocumentStream is the slice of evidence.service methods needed to
// retrieve document bytes. It mirrors the methods on evidence.Service used by
// the AI document content adapter.
type EvidenceDocumentStream interface {
	GetDocumentStream(ctx context.Context, orgID, evidenceID, documentID, downloaderID uuid.UUID) (*evidenceapp.DownloadDocumentResult, error)
}

// DocumentContentProvider implements aiapplication.DocumentContentProvider by
// delegating to the evidence service's document streaming endpoint. Tenant
// isolation and audit are enforced by the evidence service.
type DocumentContentProvider struct {
	evidence EvidenceDocumentStream
}

func NewDocumentContentProvider(evidence EvidenceDocumentStream) *DocumentContentProvider {
	return &DocumentContentProvider{evidence: evidence}
}

func (p *DocumentContentProvider) GetDocumentContent(ctx context.Context, orgID, evidenceID, documentID, actorID uuid.UUID) (io.ReadCloser, error) {
	result, err := p.evidence.GetDocumentStream(ctx, orgID, evidenceID, documentID, actorID)
	if err != nil {
		return nil, fmt.Errorf("failed to stream document: %w", err)
	}
	return result.Content, nil
}
