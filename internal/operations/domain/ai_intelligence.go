package domain

import (
	"context"
	"time"

	"github.com/alrazihi/civora/internal/ai/domain"
	"github.com/google/uuid"
)

type IntelligenceRequestType string

const (
	IntelligenceRequestTypeSummarizeTrends      IntelligenceRequestType = "summarize_trends"
	IntelligenceRequestTypeExplainBottlenecks   IntelligenceRequestType = "explain_bottlenecks"
	IntelligenceRequestTypeSummarizeWorkload    IntelligenceRequestType = "summarize_workload"
	IntelligenceRequestTypeIdentifyAnomalies    IntelligenceRequestType = "identify_anomalies"
	IntelligenceRequestTypeAnswerQuestion       IntelligenceRequestType = "answer_question"
	IntelligenceRequestTypeSuggestQuestions     IntelligenceRequestType = "suggest_questions"
	IntelligenceRequestTypeExplainChanges       IntelligenceRequestType = "explain_changes"
)

type IntelligenceRequest struct {
	Type        IntelligenceRequestType
	Question    string
	Period      MetricPeriod
	Bucket      time.Time
	WorkflowKey string
	Status      string
	Language    string
}

type IntelligenceResponse struct {
	Type               IntelligenceRequestType
	Summary            string
	Details            []IntelligenceDetail
	SuggestedQuestions []string
	Model              domain.ModelInfo
	GeneratedAt        time.Time
}

type IntelligenceDetail struct {
	Label      string
	Value      string
	Source     string
	ChangeFrom *string
	Confidence float64
}

type IntelligenceProvider interface {
	GenerateChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	ProviderInfo() domain.ModelInfo
}

type ChatRequest struct {
	OrganizationID uuid.UUID
	Messages       []ChatMessage
	MaxTokens      int
	Temperature    float64
}

type ChatMessage struct {
	Role    string
	Content string
}

type ChatResponse struct {
	Content           string
	Model             domain.ModelInfo
	InputHash         string
	OutputHash        string
	PromptTokens      int
	CompletionTokens  int
}
