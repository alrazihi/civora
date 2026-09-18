package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/alrazihi/civora/internal/ai/application"
	aiDomain "github.com/alrazihi/civora/internal/ai/domain"
	"github.com/alrazihi/civora/internal/operations/domain"
	"github.com/google/uuid"
)

type OperationsIntelligenceService struct {
	aiProvider         application.AIProvider
	operationsService  *OperationsService
	analysisService    domain.AnalysisService
	impactService      domain.ImpactService
}

func NewOperationsIntelligenceService(
	aiProvider application.AIProvider,
	operationsService *OperationsService,
	analysisService domain.AnalysisService,
	impactService domain.ImpactService,
) *OperationsIntelligenceService {
	return &OperationsIntelligenceService{
		aiProvider:        aiProvider,
		operationsService: operationsService,
		analysisService:   analysisService,
		impactService:     impactService,
	}
}

func (s *OperationsIntelligenceService) GenerateIntelligence(ctx context.Context, orgID uuid.UUID, req domain.IntelligenceRequest) (*domain.IntelligenceResponse, error) {
	if s.aiProvider == nil {
		return nil, fmt.Errorf("AI provider is not configured")
	}

	modelInfo := s.aiProvider.ProviderInfo()

	messages, grounding, err := s.buildGroundedMessages(ctx, orgID, req)
	if err != nil {
		return nil, err
	}

	chatReq := application.ChatRequest{
		OrganizationID: orgID,
		Messages:       messages,
		MaxTokens:      1024,
		Temperature:    0.2,
	}

	resp, err := s.aiProvider.GenerateChatCompletion(ctx, chatReq)
	if err != nil {
		return nil, fmt.Errorf("AI provider unavailable: %w", err)
	}

	outputHash := resp.OutputHash
	if outputHash == "" {
		outputHash = computeSHA256(resp.Content)
	}

	response := &domain.IntelligenceResponse{
		Type:        req.Type,
		Summary:     resp.Content,
		Model:       aiDomain.ModelInfo(modelInfo),
		GeneratedAt: time.Now().UTC(),
		Details:     grounding,
	}

	return response, nil
}

func computeSHA256(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

func (s *OperationsIntelligenceService) buildGroundedMessages(ctx context.Context, orgID uuid.UUID, req domain.IntelligenceRequest) ([]application.ChatMessage, []domain.IntelligenceDetail, error) {
	bucketStart := domain.BucketStart(req.Bucket, req.Period)

	var details []domain.IntelligenceDetail

	var sb strings.Builder
	sb.WriteString("You are an operations intelligence assistant for CIVORA. ")
	sb.WriteString("You must ONLY use the provided metrics data. ")
	sb.WriteString("Do NOT invent metrics, alter workflows, or make unauthorized changes. ")
	sb.WriteString("If data is insufficient, say so explicitly. ")
	sb.WriteString("All statements must be traceable to the provided data.\n\n")

	switch req.Type {
	case domain.IntelligenceRequestTypeSummarizeTrends:
		sb.WriteString("Summarize the operational trends visible in the following metrics data. ")
		sb.WriteString("Focus on case volume, throughput, and decision patterns. ")
	case domain.IntelligenceRequestTypeExplainBottlenecks:
		sb.WriteString("Explain any bottlenecks observed in the following metrics data. ")
		sb.WriteString("Identify states with unusually long durations, high pending reviews, or workflow closures that deviate from expected patterns. ")
	case domain.IntelligenceRequestTypeSummarizeWorkload:
		sb.WriteString("Summarize the current workload based on the following metrics data. ")
		sb.WriteString("Cover open cases, aging cases, pending reviews, and assistance outcomes. ")
	case domain.IntelligenceRequestTypeIdentifyAnomalies:
		sb.WriteString("Identify any unusual patterns or anomalies in the following metrics data. ")
		sb.WriteString("Compare current values against typical ranges and flag anything that stands out. ")
	case domain.IntelligenceRequestTypeAnswerQuestion:
		sb.WriteString("Answer the following question using ONLY the provided metrics data. ")
		sb.WriteString("If the answer cannot be derived from the data, say so. ")
		sb.WriteString("Question: " + req.Question + "\n\n")
	case domain.IntelligenceRequestTypeSuggestQuestions:
		sb.WriteString("Based on the following metrics data, suggest 3-5 useful questions an administrator might want to explore further. ")
	case domain.IntelligenceRequestTypeExplainChanges:
		sb.WriteString("Explain any significant changes visible in the following metrics data compared to typical baselines. ")
	default:
		sb.WriteString("Provide an analysis of the following metrics data. ")
	}

	sb.WriteString("\n\nAuthorized Metrics Data (JSON):\n")

	metrics, err := s.operationsService.GetAllMetrics(ctx, orgID, req.Period, bucketStart)
	if err == nil {
		sanitized := sanitizeMetricsForExport(metrics)
		metricsJSON, _ := json.MarshalIndent(sanitized, "", "  ")
		sb.WriteString(string(metricsJSON))
		sb.WriteString("\n")
		details = append(details, domain.IntelligenceDetail{
			Label:      "operations_metrics",
			Value:      "included",
			Source:     "operations_service",
			Confidence: 1.0,
		})
	} else {
		sb.WriteString("{ \"error\": \"operations metrics unavailable\" }\n")
		details = append(details, domain.IntelligenceDetail{
			Label:      "operations_metrics",
			Value:      "unavailable",
			Source:     "operations_service",
			Confidence: 0.0,
		})
	}

	thresholds := domain.DefaultAnalysisThresholds
	report, err := s.analysisService.GetWorkflowAnalysisReport(ctx, orgID, thresholds)
	if err == nil {
		sanitized := sanitizeAnalysisForExport(report)
		analysisJSON, _ := json.MarshalIndent(sanitized, "", "  ")
		sb.WriteString("\nWorkflow Analysis Data (JSON):\n")
		sb.WriteString(string(analysisJSON))
		sb.WriteString("\n")
		details = append(details, domain.IntelligenceDetail{
			Label:      "workflow_analysis",
			Value:      "included",
			Source:     "analysis_service",
			Confidence: 1.0,
		})
	} else {
		sb.WriteString("\n{ \"error\": \"workflow analysis unavailable\" }\n")
		details = append(details, domain.IntelligenceDetail{
			Label:      "workflow_analysis",
			Value:      "unavailable",
			Source:     "analysis_service",
			Confidence: 0.0,
		})
	}

	impactReport, err := s.impactService.GetImpactIntelligenceReport(ctx, orgID, req.Period, bucketStart)
	if err == nil {
		sanitized := sanitizeImpactForExport(impactReport)
		impactJSON, _ := json.MarshalIndent(sanitized, "", "  ")
		sb.WriteString("\nImpact Intelligence Data (JSON):\n")
		sb.WriteString(string(impactJSON))
		sb.WriteString("\n")
		details = append(details, domain.IntelligenceDetail{
			Label:      "impact_intelligence",
			Value:      "included",
			Source:     "impact_service",
			Confidence: 1.0,
		})
	} else {
		sb.WriteString("\n{ \"error\": \"impact intelligence unavailable\" }\n")
		details = append(details, domain.IntelligenceDetail{
			Label:      "impact_intelligence",
			Value:      "unavailable",
			Source:     "impact_service",
			Confidence: 0.0,
		})
	}

	messages := []application.ChatMessage{
		{Role: "system", Content: sb.String()},
	}

	if req.Question != "" {
		messages = append(messages, application.ChatMessage{
			Role:    "user",
			Content: req.Question,
		})
	} else {
		messages = append(messages, application.ChatMessage{
			Role:    "user",
			Content: fmt.Sprintf("Provide a %s analysis based on the data above.", req.Type),
		})
	}

	return messages, details, nil
}
