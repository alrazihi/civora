package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alrazihi/civora/internal/ai/application"
	"github.com/alrazihi/civora/internal/ai/domain"
	"github.com/google/uuid"
)

func TestLocalProvider_ProviderInfo(t *testing.T) {
	p := NewLocalProvider("http://localhost:11434/v1", "llama3")

	info := p.ProviderInfo()
	if info.Name != "llama3" {
		t.Errorf("expected model 'llama3', got %s", info.Name)
	}
	if info.Provider != "local" {
		t.Errorf("expected provider 'local', got %s", info.Provider)
	}
}

func TestLocalProvider_GenerateObservations_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Errorf("expected no Authorization header for local provider, got %s", r.Header.Get("Authorization"))
		}

		var req openAIRequest
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &req); err != nil {
			t.Errorf("failed to unmarshal request: %v", err)
		}

		if req.Model != "llama3" {
			t.Errorf("expected model 'llama3', got %s", req.Model)
		}

		resp := openAIResponse{
			ID: "test-id",
			Choices: []openAIChoice{
				{
					Message: openAIMessage{
						Role: "assistant",
						Content: json.RawMessage(`{
							"observations": [
								{
									"type": "SUMMARY",
									"content": {"text": "Local LLM summary of the evidence"},
									"confidence": 0.85
								}
							]
						}`),
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewLocalProvider(server.URL, "llama3")

	req := application.ProviderRequest{
		OrganizationID: uuid.New(),
		EvidenceID:     uuid.New(),
		Documents: []domain.DocumentContent{
			{
				DocumentID:  uuid.New(),
				FileName:    "test.txt",
				ContentType: "text/plain",
				Content:     []byte("This is test document content for analysis."),
				Checksum:    "abc123",
			},
		},
		Options: domain.ProviderOptions{
			Types:     []domain.ObservationType{domain.ObservationTypeSummary},
			MaxTokens: 4096,
		},
	}

	results, err := p.GenerateObservations(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 observation, got %d", len(results))
	}

	if results[0].Type != domain.ObservationTypeSummary {
		t.Errorf("expected observation type SUMMARY, got %s", results[0].Type)
	}

	if results[0].Model.Provider != "local" {
		t.Errorf("expected model provider 'local', got %s", results[0].Model.Provider)
	}

	if results[0].InputHash == "" || results[0].OutputHash == "" {
		t.Error("expected non-empty input and output hashes")
	}
}

func TestLocalProvider_GenerateObservations_NoDocuments(t *testing.T) {
	p := NewLocalProvider("http://localhost:11434/v1", "llama3")

	_, err := p.GenerateObservations(context.Background(), application.ProviderRequest{
		OrganizationID: uuid.New(),
		EvidenceID:     uuid.New(),
	})

	if err == nil {
		t.Fatal("expected error for no documents")
	}
}

func TestLocalProvider_GenerateObservations_APIErrors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantErr    bool
	}{
		{
			name:       "rate limited",
			statusCode: 429,
			body:       `{"error": {"message": "rate limit exceeded", "type": "rate_limit"}}`,
			wantErr:    true,
		},
		{
			name:       "server error",
			statusCode: 500,
			body:       `{"error": {"message": "internal server error", "type": "server_error"}}`,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			p := NewLocalProvider(server.URL, "llama3")

			_, err := p.GenerateObservations(context.Background(), application.ProviderRequest{
				OrganizationID: uuid.New(),
				EvidenceID:     uuid.New(),
				Documents: []domain.DocumentContent{
					{DocumentID: uuid.New(), FileName: "test.txt", ContentType: "text/plain", Content: []byte("test"), Checksum: "abc"},
				},
			})

			if (err != nil) != tt.wantErr {
				t.Errorf("expected error: %v, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestLocalProvider_DefaultValues(t *testing.T) {
	p := NewLocalProvider("", "")

	info := p.ProviderInfo()
	if info.Name != "llama3" {
		t.Errorf("expected default model 'llama3', got %s", info.Name)
	}
	if info.Provider != "local" {
		t.Errorf("expected provider 'local', got %s", info.Provider)
	}

	if p.baseURL != "http://localhost:11434/v1" {
		t.Errorf("expected default base URL, got %s", p.baseURL)
	}
}

func TestLocalProvider_GenerateObservations_EmptyObservations(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openAIResponse{
			ID: "test-id",
			Choices: []openAIChoice{
				{
					Message: openAIMessage{
						Role:    "assistant",
						Content: json.RawMessage(`{"observations": []}`),
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewLocalProvider(server.URL, "llama3")

	req := application.ProviderRequest{
		OrganizationID: uuid.New(),
		EvidenceID:     uuid.New(),
		Documents: []domain.DocumentContent{
			{DocumentID: uuid.New(), FileName: "test.txt", ContentType: "text/plain", Content: []byte("test"), Checksum: "abc"},
		},
		Options: domain.ProviderOptions{
			MaxTokens: 4096,
		},
	}

	results, err := p.GenerateObservations(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 observations, got %d", len(results))
	}
}

func TestLocalProvider_ImplementsAIProvider(t *testing.T) {
	var _ application.AIProvider = (*LocalProvider)(nil)
}

func TestOpenAIProvider_GenerateObservations_SkipAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Errorf("expected no Authorization header with SkipAuth, got %s", r.Header.Get("Authorization"))
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		resp := openAIResponse{
			ID: "test-id",
			Choices: []openAIChoice{
				{
					Message: openAIMessage{
						Role:    "assistant",
						Content: json.RawMessage(`{"observations": []}`),
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewOpenAIProvider(OpenAIProviderConfig{
		APIKey:   "",
		Model:    "gpt-4o-mini",
		BaseURL:  server.URL,
		SkipAuth: true,
	})

	_, err := p.GenerateObservations(context.Background(), application.ProviderRequest{
		OrganizationID: uuid.New(),
		EvidenceID:     uuid.New(),
		Documents: []domain.DocumentContent{
			{DocumentID: uuid.New(), FileName: "test.txt", ContentType: "text/plain", Content: []byte("test"), Checksum: "abc"},
		},
	})

	if err != nil {
		t.Fatalf("expected no error with SkipAuth, got: %v", err)
	}
}

func TestOpenAIProvider_GenerateObservations_SkipAuthDisabledNoKey(t *testing.T) {
	p := NewOpenAIProvider(OpenAIProviderConfig{
		APIKey:   "",
		Model:    "gpt-4o-mini",
		SkipAuth: false,
	})

	_, err := p.GenerateObservations(context.Background(), application.ProviderRequest{
		OrganizationID: uuid.New(),
		EvidenceID:     uuid.New(),
		Documents: []domain.DocumentContent{
			{DocumentID: uuid.New(), FileName: "test.txt", ContentType: "text/plain", Content: []byte("test"), Checksum: "abc"},
		},
	})

	if err == nil {
		t.Fatal("expected error for missing API key without SkipAuth")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Errorf("expected 'not configured' error, got: %v", err)
	}
}
