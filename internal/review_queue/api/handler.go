package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/alrazihi/civora/internal/cases/domain"
	"github.com/alrazihi/civora/internal/middleware"
	reviewapp "github.com/alrazihi/civora/internal/review_queue/application"
	reviewdomain "github.com/alrazihi/civora/internal/review_queue/domain"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type ReviewQueueService interface {
	GetQueue(ctx context.Context, params reviewapp.GetQueueParams) ([]*reviewdomain.ReviewQueueEntry, int, error)
	ClaimReview(ctx context.Context, params reviewapp.ClaimReviewParams) (*reviewdomain.ReviewQueueEntry, error)
	StartReview(ctx context.Context, params reviewapp.StartReviewParams) (*reviewdomain.ReviewQueueEntry, error)
	CompleteReview(ctx context.Context, params reviewapp.CompleteReviewParams) (*reviewdomain.ReviewQueueEntry, error)
	EscalateReview(ctx context.Context, params reviewapp.EscalateReviewParams) (*reviewdomain.ReviewQueueEntry, error)
	RequestInformation(ctx context.Context, params reviewapp.RequestInformationParams) (*reviewdomain.ReviewQueueEntry, error)
	GetReview(ctx context.Context, orgID, reviewID uuid.UUID) (*reviewdomain.ReviewQueueEntry, error)
	Save(ctx context.Context, entry *reviewdomain.ReviewQueueEntry) error
}

type Handler struct {
	svc ReviewQueueService
}

func NewHandler(svc ReviewQueueService) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(r chi.Router, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/v1/organizations/{orgId}/review-queue", func(r chi.Router) {
		r.Use(authMiddleware)
		r.Use(middleware.RequireSameTenant)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAnyRole("admin", "staff"))
			r.Get("/", h.GetQueue)
			r.Post("/", h.CreateReview)
			r.Get("/{reviewId}", h.GetReview)
			r.Post("/{reviewId}/claim", h.ClaimReview)
			r.Post("/{reviewId}/start", h.StartReview)
			r.Post("/{reviewId}/complete", h.CompleteReview)
			r.Post("/{reviewId}/escalate", h.EscalateReview)
			r.Post("/{reviewId}/request-information", h.RequestInformation)
		})
	})
}

func (h *Handler) GetQueue(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	statusStr := r.URL.Query().Get("status")
	var status *reviewdomain.ReviewStatus
	if statusStr != "" {
		s := reviewdomain.ReviewStatus(statusStr)
		if err := reviewdomain.ValidateReviewStatus(s); err != nil {
			shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid status")
			return
		}
		status = &s
	}

	assignedToStr := r.URL.Query().Get("assigned_to_me")
	var assignedToID *uuid.UUID
	if assignedToStr == "true" {
		userID := getUserID(r)
		if userID == uuid.Nil {
			shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
			return
		}
		assignedToID = &userID
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 200 {
		perPage = 200
	}
	offset := (page - 1) * perPage

	items, total, err := h.svc.GetQueue(r.Context(), reviewapp.GetQueueParams{
		OrganizationID: orgID,
		Status:         status,
		AssignedToID:   assignedToID,
		Limit:          perPage,
		Offset:         offset,
	})
	if err != nil {
		writeReviewError(w, err)
		return
	}

	result := make([]map[string]interface{}, len(items))
	for i, entry := range items {
		result[i] = serializeReviewEntry(entry)
	}

	shared.WritePaginatedSuccess(w, http.StatusOK, result, page, perPage, total)
}

func (h *Handler) CreateReview(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	var req struct {
		CaseID              string   `json:"case_id"`
		WorkflowInstanceID   string   `json:"workflow_instance_id"`
		Priority            string   `json:"priority"`
		WorkflowState       string   `json:"workflow_state"`
		RuleEvaluationIDs   []string `json:"rule_evaluation_ids"`
		MissingInformation  []string `json:"missing_information"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid request body")
		return
	}

	caseID, err := uuid.Parse(req.CaseID)
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid case ID")
		return
	}

	workflowInstanceID, err := uuid.Parse(req.WorkflowInstanceID)
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid workflow instance ID")
		return
	}

	var ruleEvalIDs []uuid.UUID
	for _, idStr := range req.RuleEvaluationIDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid rule evaluation ID")
			return
		}
		ruleEvalIDs = append(ruleEvalIDs, id)
	}

	entry := reviewdomain.NewReviewQueueEntry(orgID, caseID, workflowInstanceID, req.WorkflowState, domain.Priority(req.Priority), ruleEvalIDs)
	entry.MissingInformation = req.MissingInformation
	if entry.Priority == "" {
		entry.Priority = "NORMAL"
	}

	if err := h.svc.Save(r.Context(), entry); err != nil {
		writeReviewError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusCreated, serializeReviewEntry(entry), nil)
}

func (h *Handler) GetReview(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	reviewID, ok := parseUUID(r, "reviewId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid review ID")
		return
	}

	entry, err := h.svc.GetReview(r.Context(), orgID, reviewID)
	if err != nil {
		writeReviewError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeReviewEntry(entry), nil)
}

func (h *Handler) ClaimReview(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	reviewID, ok := parseUUID(r, "reviewId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid review ID")
		return
	}

	reviewerID := getUserID(r)
	if reviewerID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}

	entry, err := h.svc.ClaimReview(r.Context(), reviewapp.ClaimReviewParams{
		OrganizationID: orgID,
		ReviewID:       reviewID,
		ReviewerID:     reviewerID,
	})
	if err != nil {
		writeReviewError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeReviewEntry(entry), nil)
}

func (h *Handler) StartReview(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	reviewID, ok := parseUUID(r, "reviewId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid review ID")
		return
	}

	reviewerID := getUserID(r)
	if reviewerID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}

	entry, err := h.svc.StartReview(r.Context(), reviewapp.StartReviewParams{
		OrganizationID: orgID,
		ReviewID:       reviewID,
		ReviewerID:     reviewerID,
	})
	if err != nil {
		writeReviewError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeReviewEntry(entry), nil)
}

func (h *Handler) CompleteReview(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	reviewID, ok := parseUUID(r, "reviewId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid review ID")
		return
	}

	reviewerID := getUserID(r)
	if reviewerID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}

	var req struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid request body")
		return
	}

	entry, err := h.svc.CompleteReview(r.Context(), reviewapp.CompleteReviewParams{
		OrganizationID: orgID,
		ReviewID:       reviewID,
		ReviewerID:     reviewerID,
		Decision:       req.Decision,
		Reason:         req.Reason,
	})
	if err != nil {
		writeReviewError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeReviewEntry(entry), nil)
}

func (h *Handler) EscalateReview(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	reviewID, ok := parseUUID(r, "reviewId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid review ID")
		return
	}

	reviewerID := getUserID(r)
	if reviewerID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid request body")
		return
	}

	entry, err := h.svc.EscalateReview(r.Context(), reviewapp.EscalateReviewParams{
		OrganizationID: orgID,
		ReviewID:       reviewID,
		ReviewerID:     reviewerID,
		Reason:         req.Reason,
	})
	if err != nil {
		writeReviewError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeReviewEntry(entry), nil)
}

func (h *Handler) RequestInformation(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	reviewID, ok := parseUUID(r, "reviewId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid review ID")
		return
	}

	reviewerID := getUserID(r)
	if reviewerID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}

	var req struct {
		MissingFields []string `json:"missing_fields"`
		Reason        string   `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid request body")
		return
	}

	entry, err := h.svc.RequestInformation(r.Context(), reviewapp.RequestInformationParams{
		OrganizationID: orgID,
		ReviewID:       reviewID,
		ReviewerID:     reviewerID,
		MissingFields:  req.MissingFields,
		Reason:         req.Reason,
	})
	if err != nil {
		writeReviewError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeReviewEntry(entry), nil)
}

func parseUUID(r *http.Request, name string) (uuid.UUID, bool) {
	v := chi.URLParam(r, name)
	id, err := uuid.Parse(v)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

func getUserID(r *http.Request) uuid.UUID {
	idStr := middleware.GetUserID(r)
	if idStr == "" {
		return uuid.Nil
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return uuid.Nil
	}
	return id
}

func serializeReviewEntry(entry *reviewdomain.ReviewQueueEntry) map[string]interface{} {
	result := map[string]interface{}{
		"id":                  entry.ID,
		"organization_id":     entry.OrganizationID,
		"case_id":             entry.CaseID,
		"workflow_instance_id": entry.WorkflowInstanceID,
		"status":              entry.Status,
		"priority":            entry.Priority,
		"workflow_state":      entry.WorkflowState,
		"rule_evaluation_ids": entry.RuleEvaluationIDs,
		"missing_information": entry.MissingInformation,
		"created_at":          entry.CreatedAt,
		"updated_at":          entry.UpdatedAt,
		"metadata":            entry.Metadata,
	}
	if entry.AssignedToID != nil {
		result["assigned_to"] = entry.AssignedToID
	}
	if entry.CompletedAt != nil {
		result["completed_at"] = entry.CompletedAt
	}
	return result
}

func writeReviewError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, reviewapp.ErrReviewNotFound):
		shared.WriteError(w, http.StatusNotFound, shared.CodeNotFound, "review queue entry not found")
	case errors.Is(err, reviewapp.ErrReviewNotPending):
		shared.WriteError(w, http.StatusConflict, "CONFLICT", "review is not pending")
	case errors.Is(err, reviewapp.ErrReviewNotAssigned):
		shared.WriteError(w, http.StatusForbidden, "FORBIDDEN", "review is not assigned to you")
	case errors.Is(err, reviewapp.ErrReviewNotInReview):
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "review is not in review")
	case errors.Is(err, reviewapp.ErrReviewAlreadyFinal):
		shared.WriteError(w, http.StatusConflict, "CONFLICT", "review is already completed")
	case errors.Is(err, reviewapp.ErrCaseNotFound):
		shared.WriteError(w, http.StatusNotFound, shared.CodeNotFound, "case not found")
	case errors.Is(err, reviewapp.ErrUserNotFound):
		shared.WriteError(w, http.StatusForbidden, "FORBIDDEN", "user not found")
	default:
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "internal server error")
	}
}
