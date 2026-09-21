package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alrazihi/civora/internal/documentintelligence/application"
	"github.com/alrazihi/civora/internal/documentintelligence/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAIProvider_GenerateDocumentAnalyses_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/chat/completions", r.URL.Path)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		response := map[string]interface{}{
			"id": "test-id",
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"role": "assistant",
						"content": map[string]interface{}{
							"analyses": []map[string]interface{}{
								{
									"type":       "CLASSIFICATION",
									"content":    map[string]interface{}{"classification": "employment_contract"},
									"confidence": 0.9,
								},
							},
						},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(response) //nolint:gosec // test mock response
	}))
	defer server.Close()

	cfg := OpenAIProviderConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
		Model:   "gpt-4o-mini",
	}
	provider := NewOpenAIProvider(cfg)

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
		Options: domain.ProviderOptions{
			Types:     []domain.AnalysisType{domain.AnalysisTypeClassification},
			MaxTokens: 4096,
		},
	}

	results, err := provider.GenerateDocumentAnalyses(context.Background(), req)

	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, domain.AnalysisTypeClassification, results[0].Type)
	assert.Equal(t, "employment_contract", results[0].Content["classification"])
	assert.InDelta(t, 0.9, *results[0].Confidence, 0.001)
}

func TestOpenAIProvider_GenerateDocumentAnalyses_NoAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("Authorization"))

		response := map[string]interface{}{
			"id": "test-id",
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"role": "assistant",
						"content": map[string]interface{}{
							"analyses": []map[string]interface{}{
								{
									"type":       "SUMMARIZATION",
									"content":    map[string]interface{}{"summary": "test summary"},
									"confidence": 0.8,
								},
							},
						},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(response) //nolint:gosec // test mock response
	}))
	defer server.Close()

	cfg := OpenAIProviderConfig{
		BaseURL:  server.URL,
		Model:    "gpt-4o-mini",
		SkipAuth: true,
	}
	provider := NewOpenAIProvider(cfg)

	req := application.ProviderRequest{
		OrganizationID: uuid.New(),
		EvidenceID:     uuid.New(),
		Documents: []domain.DocumentContent{
			{
				DocumentID:  uuid.New(),
				FileName:    "test.txt",
				ContentType: "text/plain",
				Content:     []byte("test content"),
				Checksum:    "abc123",
			},
		},
		Options: domain.ProviderOptions{
			Types:     []domain.AnalysisType{domain.AnalysisTypeSummarization},
			MaxTokens: 4096,
		},
	}

	results, err := provider.GenerateDocumentAnalyses(context.Background(), req)

	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, domain.AnalysisTypeSummarization, results[0].Type)
}

func TestOpenAIProvider_GenerateDocumentAnalyses_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid API key"}) //nolint:gosec // test mock response
	}))
	defer server.Close()

	cfg := OpenAIProviderConfig{
		APIKey:  "invalid-key",
		BaseURL: server.URL,
		Model:   "gpt-4o-mini",
	}
	provider := NewOpenAIProvider(cfg)

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
		Options: domain.ProviderOptions{
			Types:     []domain.AnalysisType{domain.AnalysisTypeClassification},
			MaxTokens: 4096,
		},
	}

	_, err := provider.GenerateDocumentAnalyses(context.Background(), req)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "OpenAI API error")
}

func TestOpenAIProvider_GenerateDocumentAnalyses_InvalidResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"invalid": "response"}) //nolint:gosec // test mock response
	}))
	defer server.Close()

	cfg := OpenAIProviderConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
		Model:   "gpt-4o-mini",
	}
	provider := NewOpenAIProvider(cfg)

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
		Options: domain.ProviderOptions{
			Types:     []domain.AnalysisType{domain.AnalysisTypeClassification},
			MaxTokens: 4096,
		},
	}

	_, err := provider.GenerateDocumentAnalyses(context.Background(), req)

	assert.Error(t, err)
}

func TestOpenAIProvider_GenerateDocumentAnalyses_NoDocuments(t *testing.T) {
	cfg := OpenAIProviderConfig{
		APIKey:  "test-key",
		BaseURL: "http://localhost:11434/v1",
		Model:   "gpt-4o-mini",
	}
	provider := NewOpenAIProvider(cfg)

	req := application.ProviderRequest{
		OrganizationID: uuid.New(),
		EvidenceID:     uuid.New(),
		Documents:      []domain.DocumentContent{},
		Options:        domain.ProviderOptions{},
	}

	_, err := provider.GenerateDocumentAnalyses(context.Background(), req)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no documents provided")
}
