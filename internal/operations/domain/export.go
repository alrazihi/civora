package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ExportFormat string

const (
	ExportFormatJSON   ExportFormat = "json"
	ExportFormatCSV    ExportFormat = "csv"
	ExportFormatReport ExportFormat = "report"
)

type ExportScope string

const (
	ExportScopeOperations ExportScope = "operations"
	ExportScopeAnalysis   ExportScope = "analysis"
	ExportScopeImpact     ExportScope = "impact"
	ExportScopeAll        ExportScope = "all"
)

type ExportRequest struct {
	Format      ExportFormat
	Scope       ExportScope
	Period      MetricPeriod
	Bucket      time.Time
	WorkflowKey string
	Status      string
}

type ExportMetadata struct {
	GeneratedAt       time.Time         `json:"generated_at"`
	OrganizationID    uuid.UUID         `json:"organization_id"`
	OrganizationScope string            `json:"organization_scope"`
	Filters           map[string]string `json:"filters"`
	MetricVersion     string            `json:"metric_version"`
	DataSources       []string          `json:"data_sources"`
}

type ExportResult struct {
	ContentType string
	Filename    string
	Body        []byte
	Metadata    ExportMetadata
}

type ExportService interface {
	GenerateExport(ctx context.Context, orgID uuid.UUID, req ExportRequest) (*ExportResult, error)
}

const ExportMetricVersion = "0.9.0"

var SensitiveFieldPatterns = []string{
	"password",
	"secret",
	"token",
	"api_key",
	"apikey",
	"private_key",
	"path",
	"filepath",
	"directory",
	"home_dir",
	"working_dir",
	"internal_path",
	"storage_path",
	"database_url",
	"dsn",
	"credential",
	"hash",
	"salt",
	"session_id",
	"jti",
	"jwt",
	"bearer",
	"authorization",
	"cookie",
	"csrf",
	"hmac",
	"signature",
}

func SanitizeExportData(data interface{}) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		sanitized := make(map[string]interface{}, len(v))
		for k, val := range v {
			if isSensitiveField(k) {
				sanitized[k] = "[REDACTED]"
				continue
			}
			sanitized[k] = SanitizeExportData(val)
		}
		return sanitized
	case []interface{}:
		sanitized := make([]interface{}, len(v))
		for i, item := range v {
			sanitized[i] = SanitizeExportData(item)
		}
		return sanitized
	default:
		return data
	}
}

func isSensitiveField(name string) bool {
	lower := strings.ToLower(name)
	for _, pattern := range SensitiveFieldPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

func BuildExportMetadata(orgID uuid.UUID, scope ExportScope, period MetricPeriod, bucket time.Time, workflowKey, status string, dataSources []string) ExportMetadata {
	filters := map[string]string{
		"period": string(period),
		"bucket": bucket.Format(time.RFC3339),
	}
	if workflowKey != "" {
		filters["workflow_key"] = workflowKey
	}
	if status != "" {
		filters["status"] = status
	}

	return ExportMetadata{
		GeneratedAt:       time.Now().UTC(),
		OrganizationID:    orgID,
		OrganizationScope: fmt.Sprintf("organization:%s", orgID),
		Filters:           filters,
		MetricVersion:     ExportMetricVersion,
		DataSources:       dataSources,
	}
}

func (r *ExportResult) ToJSON() ([]byte, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(r.Body, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse export data for JSON: %w", err)
	}
	envelope := map[string]interface{}{
		"metadata": r.Metadata,
		"data":     SanitizeExportData(raw),
	}
	return json.MarshalIndent(envelope, "", "  ")
}

func (r *ExportResult) ToCSV() ([]byte, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(r.Body, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse export data for CSV: %w", err)
	}
	sanitizedMap, _ := SanitizeExportData(raw).(map[string]interface{})
	if sanitizedMap == nil {
		return nil, fmt.Errorf("invalid export data structure for CSV")
	}
	return FlattenToCSV(sanitizedMap), nil
}

func FlattenToCSV(data map[string]interface{}) []byte {
	var b strings.Builder
	b.WriteString("section,metric,value,unit,category\n")

	writeSection := func(section string, metrics []interface{}) {
		for _, m := range metrics {
			metric, ok := m.(map[string]interface{})
			if !ok {
				continue
			}
			category := fmt.Sprintf("%v", metric["category"])
			name := fmt.Sprintf("%v", metric["name"])
			unit := fmt.Sprintf("%v", metric["unit"])
			value := ""
			if category == "impact" {
				if impact, ok := metric["impact_value"].(float64); ok {
					value = fmt.Sprintf("%.4f", impact)
				}
			} else {
				if outcome, ok := metric["outcome_count"].(float64); ok && outcome > 0 {
					value = fmt.Sprintf("%.0f", outcome)
				} else if activity, ok := metric["activity_count"].(float64); ok && activity > 0 {
					value = fmt.Sprintf("%.0f", activity)
				}
			}
			b.WriteString(fmt.Sprintf("%s,%s,%s,%s,%s\n", escapeCSV(section), escapeCSV(name), escapeCSV(value), escapeCSV(unit), escapeCSV(category)))
		}
	}

	if ops, ok := data["operations_metrics"].(map[string]interface{}); ok {
		if cv, ok := ops["case_volume"].(map[string]interface{}); ok {
			b.WriteString(fmt.Sprintf("case_volume,total_cases,%d,cases,activity\n", int(cv["total_cases"].(float64))))
			if byStatus, ok := cv["by_status"].(map[string]interface{}); ok {
				for status, count := range byStatus {
					b.WriteString(fmt.Sprintf("case_volume,%s,%d,cases,activity\n", escapeCSV(status), int(count.(float64))))
				}
			}
		}
		if wt, ok := ops["workflow_throughput"].(map[string]interface{}); ok {
			b.WriteString(fmt.Sprintf("workflow_throughput,total_transitions,%d,transitions,activity\n", int(wt["total_transitions"].(float64))))
		}
		if sd, ok := ops["state_duration"].(map[string]interface{}); ok {
			b.WriteString(fmt.Sprintf("state_duration,avg_duration_hours,%.2f,hrs,outcome\n", sd["avg_duration_hours"].(float64)))
		}
		if pr, ok := ops["pending_reviews"].(map[string]interface{}); ok {
			b.WriteString(fmt.Sprintf("pending_reviews,pending,%d,reviews,activity\n", int(pr["pending_reviews"].(float64))))
		}
		if d, ok := ops["decisions"].(map[string]interface{}); ok {
			b.WriteString(fmt.Sprintf("decisions,total,%d,decisions,activity\n", int(d["total_decisions"].(float64))))
			b.WriteString(fmt.Sprintf("decisions,approved,%d,decisions,outcome\n", int(d["approved"].(float64))))
		}
		if ao, ok := ops["assistance_outcomes"].(map[string]interface{}); ok {
			b.WriteString(fmt.Sprintf("assistance_outcomes,total,%d,records,activity\n", int(ao["total_assistance"].(float64))))
			b.WriteString(fmt.Sprintf("assistance_outcomes,completed,%d,records,outcome\n", int(ao["completed"].(float64))))
		}
		if ir, ok := ops["information_required"].(map[string]interface{}); ok {
			b.WriteString(fmt.Sprintf("information_required,total,%d,cases,activity\n", int(ir["total_cases"].(float64))))
		}
	}

	if analysis, ok := data["workflow_analysis"].(map[string]interface{}); ok {
		if sa, ok := analysis["state_accumulations"].([]interface{}); ok {
			writeSection("state_accumulations", sa)
		}
		if sda, ok := analysis["state_duration_anomalies"].([]interface{}); ok {
			writeSection("state_duration_anomalies", sda)
		}
		if wcp, ok := analysis["workflow_closure_patterns"].([]interface{}); ok {
			writeSection("workflow_closure_patterns", wcp)
		}
		if irp, ok := analysis["information_request_patterns"].([]interface{}); ok {
			writeSection("information_request_patterns", irp)
		}
		if rbo, ok := analysis["review_backlog_observations"].([]interface{}); ok {
			writeSection("review_backlog_observations", rbo)
		}
		if te, ok := analysis["threshold_exceedances"].([]interface{}); ok {
			writeSection("threshold_exceedances", te)
		}
	}

	if impact, ok := data["impact_intelligence"].(map[string]interface{}); ok {
		if metrics, ok := impact["metrics"].([]interface{}); ok {
			writeSection("impact_metrics", metrics)
		}
	}

	return []byte(b.String())
}

func escapeCSV(s string) string {
	if strings.Contains(s, ",") || strings.Contains(s, "\"") || strings.Contains(s, "\n") {
		return "\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\""
	}
	return s
}

func (r *ExportResult) ToReport() ([]byte, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(r.Body, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse export data for report: %w", err)
	}
	sanitizedMap, _ := SanitizeExportData(raw).(map[string]interface{})

	var b strings.Builder
	b.WriteString(fmt.Sprintf("# CIVORA %s Report\n\n", strings.Title(r.Metadata.OrganizationScope)))
	b.WriteString(fmt.Sprintf("**Generated:** %s\n", r.Metadata.GeneratedAt.Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("**Metric Version:** %s\n\n", r.Metadata.MetricVersion))

	if len(r.Metadata.Filters) > 0 {
		b.WriteString("## Filters\n\n")
		for k, v := range r.Metadata.Filters {
			b.WriteString(fmt.Sprintf("- **%s:** %s\n", k, v))
		}
		b.WriteString("\n")
	}

	b.WriteString("## Data Sources\n\n")
	for _, ds := range r.Metadata.DataSources {
		b.WriteString(fmt.Sprintf("- %s\n", ds))
	}
	b.WriteString("\n")

	if ops, ok := sanitizedMap["operations_metrics"].(map[string]interface{}); ok && ops != nil {
		b.WriteString("## Operations Metrics\n\n")
		writeMetricSection(&b, "Case Volume", ops["case_volume"])
		writeMetricSection(&b, "Workflow Throughput", ops["workflow_throughput"])
		writeMetricSection(&b, "State Duration", ops["state_duration"])
		writeMetricSection(&b, "Case Cycle Time", ops["case_cycle_time"])
		writeMetricSection(&b, "Aging Cases", ops["aging_cases"])
		writeMetricSection(&b, "Pending Reviews", ops["pending_reviews"])
		writeMetricSection(&b, "Decisions", ops["decisions"])
		writeMetricSection(&b, "Assistance Outcomes", ops["assistance_outcomes"])
		writeMetricSection(&b, "Evidence Verification", ops["evidence_verification"])
		writeMetricSection(&b, "Information Required", ops["information_required"])
		b.WriteString("\n")
	}

	if analysis, ok := sanitizedMap["workflow_analysis"].(map[string]interface{}); ok && analysis != nil {
		b.WriteString("## Workflow Analysis\n\n")
		writeAnalysisSection(&b, "State Accumulations", analysis["state_accumulations"])
		writeAnalysisSection(&b, "State Duration Anomalies", analysis["state_duration_anomalies"])
		writeAnalysisSection(&b, "Workflow Closure Patterns", analysis["workflow_closure_patterns"])
		writeAnalysisSection(&b, "Information Request Patterns", analysis["information_request_patterns"])
		writeAnalysisSection(&b, "Review Backlog Observations", analysis["review_backlog_observations"])
		writeAnalysisSection(&b, "Threshold Exceedances", analysis["threshold_exceedances"])
		b.WriteString("\n")
	}

	if impact, ok := sanitizedMap["impact_intelligence"].(map[string]interface{}); ok && impact != nil {
		b.WriteString("## Impact Intelligence\n\n")
		writeImpactSection(&b, impact["metrics"])
		b.WriteString("\n")
	}

	b.WriteString("---\n\n*This report was generated automatically by CIVORA. All metrics are derived from authoritative operational data.*\n")
	return []byte(b.String()), nil
}

func writeMetricSection(b *strings.Builder, title string, data interface{}) {
	if data == nil {
		return
	}
	metric, ok := data.(map[string]interface{})
	if !ok {
		return
	}
	b.WriteString(fmt.Sprintf("### %s\n\n", title))
	b.WriteString("| Metric | Value |\n|--------|-------|\n")
	for k, v := range metric {
		if k == "organization_id" || k == "period" || k == "bucket" || k == "calculated_at" {
			continue
		}
		b.WriteString(fmt.Sprintf("| %s | %v |\n", k, v))
	}
	b.WriteString("\n")
}

func writeAnalysisSection(b *strings.Builder, title string, data interface{}) {
	if data == nil {
		return
	}
	arr, ok := data.([]interface{})
	if !ok || len(arr) == 0 {
		b.WriteString(fmt.Sprintf("### %s\n\nNo observations.\n\n", title))
		return
	}
	b.WriteString(fmt.Sprintf("### %s\n\n", title))
	b.WriteString("| Count |\n|-------|\n")
	b.WriteString(fmt.Sprintf("| %d |\n\n", len(arr)))
}

func writeImpactSection(b *strings.Builder, data interface{}) {
	if data == nil {
		return
	}
	arr, ok := data.([]interface{})
	if !ok || len(arr) == 0 {
		b.WriteString("No impact metrics.\n\n")
		return
	}
	b.WriteString("### Impact Metrics\n\n")
	b.WriteString("| Category | Metric | Value | Unit |\n|----------|--------|-------|------|\n")
	for _, m := range arr {
		metric, ok := m.(map[string]interface{})
		if !ok {
			continue
		}
		category := metric["category"]
		name := metric["name"]
		unit := metric["unit"]
		value := ""
		if category == "impact" {
			if impact, ok := metric["impact_value"].(float64); ok {
				value = fmt.Sprintf("%.4f", impact)
			}
		} else if outcome, ok := metric["outcome_count"].(float64); ok && outcome > 0 {
			value = fmt.Sprintf("%.0f", outcome)
		} else if activity, ok := metric["activity_count"].(float64); ok && activity > 0 {
			value = fmt.Sprintf("%.0f", activity)
		}
		b.WriteString(fmt.Sprintf("| %v | %v | %s | %v |\n", category, name, value, unit))
	}
	b.WriteString("\n")
}

func buildFilename(ext string, scope ExportScope, period MetricPeriod, bucket time.Time) string {
	bucketStr := bucket.Format("2006-01-02")
	return fmt.Sprintf("civora-%s-%s-%s.%s", scope, period, bucketStr, ext)
}
