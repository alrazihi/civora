package provider

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/alrazihi/civora/internal/documentintelligence/application"
	"github.com/alrazihi/civora/internal/documentintelligence/domain"
)

const (
	defaultOpenAIModel   = "gpt-4o-mini"
	defaultOpenAITimeout = 60 * time.Second
	defaultOpenAIBaseURL = "https://api.openai.com/v1"
	maxDocumentBytes     = 100_000
	maxPromptDocuments   = 10
)

type OpenAIProvider struct {
	apiKey     string
	baseURL    string
	model      string
	skipAuth   bool
	httpClient *http.Client
}

type OpenAIProviderConfig struct {
	APIKey     string
	BaseURL    string
	Model      string
	SkipAuth   bool
	HTTPClient *http.Client
}

func NewOpenAIProvider(cfg OpenAIProviderConfig) *OpenAIProvider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultOpenAIBaseURL
	}
	if cfg.Model == "" {
		cfg.Model = defaultOpenAIModel
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: defaultOpenAITimeout}
	}

	return &OpenAIProvider{
		apiKey:     cfg.APIKey,
		baseURL:    cfg.BaseURL,
		model:      cfg.Model,
		skipAuth:   cfg.SkipAuth,
		httpClient: cfg.HTTPClient,
	}
}

func (p *OpenAIProvider) ProviderInfo() domain.ModelInfo {
	return domain.ModelInfo{
		Name:     p.model,
		Version:  "1.0",
		Provider: "openai",
	}
}

type openAIRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	ResponseFmt *responseFormat `json:"response_format,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature"`
}

type openAIMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type openAIResponse struct {
	ID      string         `json:"id"`
	Choices []openAIChoice `json:"choices"`
	Usage   *openAIUsage   `json:"usage"`
	Error   *openAIError   `json:"error,omitempty"`
}

type openAIChoice struct {
	Message openAIMessage `json:"message"`
}

type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

type openAIError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type analysisResult struct {
	Type       domain.AnalysisType `json:"type"`
	Content    map[string]any      `json:"content"`
	Confidence *float64            `json:"confidence,omitempty"`
}

func (p *OpenAIProvider) GenerateDocumentAnalyses(ctx context.Context, req application.ProviderRequest) ([]application.AnalysisResult, error) {
	if !p.skipAuth && p.apiKey == "" {
		return nil, ErrProviderDisabled
	}

	if len(req.Documents) == 0 {
		return nil, fmt.Errorf("no documents provided for analysis generation")
	}

	types := req.Options.Types
	if len(types) == 0 {
		types = []domain.AnalysisType{
			domain.AnalysisTypeClassification,
			domain.AnalysisTypeSummarization,
		}
	}

	inputHash, err := p.computeInputHash(req.Documents, types, req.Options.MaxTokens)
	if err != nil {
		return nil, fmt.Errorf("failed to compute input hash: %w", err)
	}

	prompt := p.buildPrompt(types, req.Documents)
	sysPrompt := systemPrompt()

	openAIReq := openAIRequest{
		Model: p.model,
		Messages: []openAIMessage{
			{Role: "system", Content: json.RawMessage(marshalOrEmpty(sysPrompt))},
			{Role: "user", Content: json.RawMessage(marshalOrEmpty(prompt))},
		},
		ResponseFmt: &responseFormat{Type: "json_object"},
		MaxTokens:   req.Options.MaxTokens,
		Temperature: 0.3,
	}

	reqBytes, err := json.Marshal(openAIReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if !p.skipAuth {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode, string(body))
	}

	var apiResp openAIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if apiResp.Error != nil {
		return nil, fmt.Errorf("OpenAI API error: %s", apiResp.Error.Message)
	}

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	outputHash := computeSHA256(string(body))

	result, err := p.parseAnalyses(apiResp.Choices[0].Message.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse analyses: %w", err)
	}

	for i := range result {
		if result[i].InputHash == "" {
			result[i].InputHash = inputHash
		}
		if result[i].OutputHash == "" {
			result[i].OutputHash = outputHash
		}
		if result[i].Model.Name == "" {
			result[i].Model = domain.ModelInfo{
				Name:     p.model,
				Version:  "1.0",
				Provider: "openai",
			}
		}
	}

	return result, nil
}

func (p *OpenAIProvider) computeInputHash(docs []domain.DocumentContent, types []domain.AnalysisType, maxTokens int) (string, error) {
	h := sha256.New()
	for _, doc := range docs {
		h.Write([]byte(doc.DocumentID.String()))
		h.Write([]byte(doc.FileName))
		h.Write([]byte(doc.ContentType))
		h.Write([]byte(doc.Checksum))
	}
	for _, t := range types {
		h.Write([]byte(t))
	}
	h.Write([]byte(fmt.Sprintf("%d", maxTokens)))
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (p *OpenAIProvider) buildPrompt(types []domain.AnalysisType, docs []domain.DocumentContent) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("You are CIVORA's AI document intelligence assistant. Analyze the following document and produce structured analyses.\n\n"))
	sb.WriteString(fmt.Sprintf("Analysis types requested: %v\n\n", types))

	sb.WriteString("Document:\n")
	count := 0
	for i, doc := range docs {
		if i >= maxPromptDocuments {
			break
		}
		if len(doc.Content) > maxDocumentBytes {
			doc.Content = doc.Content[:maxDocumentBytes]
		}
		count++

		preview := string(doc.Content)
		if len(preview) > 5000 {
			preview = preview[:5000] + "... (truncated)"
		}

		sb.WriteString(fmt.Sprintf("--- Document %d: %s (type: %s, checksum: %s) ---\n", i+1, doc.FileName, doc.ContentType, doc.Checksum))
		sb.WriteString(preview)
		sb.WriteString("\n\n")
	}

	sb.WriteString(fmt.Sprintf("Total documents processed: %d\n\n", count))
	sb.WriteString(`Respond with a JSON object in the following format:
{
  "analyses": [
    {
      "type": "CLASSIFICATION" | "TEXT_EXTRACTION" | "STRUCTURED_EXTRACTION" | "SUMMARIZATION" | "RELEVANCE_DETECTION",
      "content": { ... analysis data ... },
      "confidence": 0.0-1.0
    }
  ]
}
Do not include any text outside the JSON object.`)

	return sb.String()
}

func systemPrompt() string {
	return `You are CIVORA's AI document intelligence assistant. You analyze case evidence documents and produce structured analyses. All analyses require human verification before use in decision-making. Do not make judgments or recommendations; only extract facts, summaries, classifications, structured data, and identify relevant information. All outputs must be grounded in the provided document content.`
}

func (p *OpenAIProvider) parseAnalyses(content json.RawMessage) ([]application.AnalysisResult, error) {
	var wrapper struct {
		Analyses []analysisResult `json:"analyses"`
	}
	if err := json.Unmarshal(content, &wrapper); err != nil {
		return nil, fmt.Errorf("failed to unmarshal analyses: %w", err)
	}

	results := make([]application.AnalysisResult, 0, len(wrapper.Analyses))
	for _, a := range wrapper.Analyses {
		if !isValidAnalysisType(a.Type) {
			continue
		}
		if a.Confidence == nil {
			defaultConf := 0.5
			a.Confidence = &defaultConf
		}
		results = append(results, application.AnalysisResult{
			Type:       a.Type,
			Content:    a.Content,
			Confidence: a.Confidence,
			Model:      p.ProviderInfo(),
		})
	}

	return results, nil
}

func isValidAnalysisType(t domain.AnalysisType) bool {
	switch t {
	case domain.AnalysisTypeClassification,
		domain.AnalysisTypeTextExtraction,
		domain.AnalysisTypeStructuredExtraction,
		domain.AnalysisTypeSummarization,
		domain.AnalysisTypeRelevanceDetection:
		return true
	default:
		return false
	}
}

func marshalOrEmpty(s string) json.RawMessage {
	b, err := json.Marshal(s)
	if err != nil {
		return json.RawMessage(`""`)
	}
	return b
}

func computeSHA256(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

var _ application.AIProvider = (*OpenAIProvider)(nil)
