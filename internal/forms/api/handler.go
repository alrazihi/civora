package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/alrazihi/civora/internal/forms/application"
	"github.com/alrazihi/civora/internal/forms/domain"
	"github.com/alrazihi/civora/internal/middleware"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	svc *application.FormService
}

func NewHandler(svc *application.FormService) *Handler {
	return &Handler{svc: svc}
}

type FormService interface {
	CreateForm(ctx context.Context, params application.CreateFormParams) (*domain.Form, error)
	UpdateForm(ctx context.Context, params application.UpdateFormParams) (*domain.Form, error)
	CreateVersion(ctx context.Context, params application.CreateVersionParams) (*domain.FormVersion, error)
	PublishVersion(ctx context.Context, params application.PublishVersionParams) (*domain.FormVersion, error)
	ArchiveForm(ctx context.Context, params application.ArchiveFormParams) (*domain.Form, error)
	GetForm(ctx context.Context, params application.GetFormParams) (*domain.Form, error)
	ListForms(ctx context.Context, params application.ListFormsParams) ([]*domain.Form, int, error)
	AddField(ctx context.Context, params application.AddFieldParams) (*domain.FormField, error)
	UpdateField(ctx context.Context, params application.UpdateFieldParams) (*domain.FormField, error)
	DeleteField(ctx context.Context, params application.DeleteFieldParams) (*domain.FormField, error)
	GetActiveVersion(ctx context.Context, params application.GetActiveVersionParams) (*domain.FormVersion, error)
}

func (h *Handler) RegisterRoutes(r chi.Router, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/v1/organizations/{orgId}/forms", func(r chi.Router) {
		r.Use(authMiddleware)
		r.Use(middleware.RequireSameTenant)
		r.Post("/", h.CreateForm)
		r.Get("/", h.ListForms)
		r.Get("/{formId}", h.GetForm)
		r.Put("/{formId}", h.UpdateForm)
		r.Post("/{formId}/versions", h.CreateVersion)
		r.Post("/{formId}/versions/{versionId}/publish", h.PublishVersion)
		r.Post("/{formId}/archive", h.ArchiveForm)
		r.Get("/{formId}/active-version", h.GetActiveVersion)
		r.Route("/{formId}/fields", func(r chi.Router) {
			r.Post("/", h.AddField)
			r.Put("/{fieldId}", h.UpdateField)
			r.Delete("/{fieldId}", h.DeleteField)
		})
	})
}

func (h *Handler) CreateForm(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	actorID := getUserID(r)
	if actorID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}

	var req struct {
		Key         string `json:"key"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid request body")
		return
	}

	form, err := h.svc.CreateForm(r.Context(), application.CreateFormParams{
		OrganizationID: orgID,
		Key:            req.Key,
		Name:           req.Name,
		Description:    req.Description,
		CreatedByID:    actorID,
	})
	if err != nil {
		writeFormError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusCreated, serializeForm(form), nil)
}

func (h *Handler) GetForm(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	formID, ok := parseUUID(r, "formId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid form ID")
		return
	}

	form, err := h.svc.GetForm(r.Context(), application.GetFormParams{
		OrganizationID: orgID,
		FormID:         formID,
	})
	if err != nil {
		writeFormError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeForm(form), nil)
}

func (h *Handler) ListForms(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	page, parseErr := strconv.Atoi(r.URL.Query().Get("page"))
	if parseErr != nil {
		page = 1
	}
	perPage, parseErr := strconv.Atoi(r.URL.Query().Get("per_page"))
	if parseErr != nil {
		perPage = 20
	}
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 200 {
		perPage = 200
	}
	offset := (page - 1) * perPage

	forms, total, err := h.svc.ListForms(r.Context(), application.ListFormsParams{
		OrganizationID: orgID,
		Limit:          perPage,
		Offset:         offset,
	})
	if err != nil {
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "failed to list forms")
		return
	}

	result := make([]map[string]interface{}, len(forms))
	for i, f := range forms {
		result[i] = serializeForm(f)
	}

	shared.WritePaginatedSuccess(w, http.StatusOK, result, page, perPage, total)
}

func (h *Handler) UpdateForm(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	formID, ok := parseUUID(r, "formId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid form ID")
		return
	}

	actorID := getUserID(r)
	if actorID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid request body")
		return
	}

	form, err := h.svc.UpdateForm(r.Context(), application.UpdateFormParams{
		OrganizationID: orgID,
		ID:             formID,
		Name:           req.Name,
		Description:    req.Description,
		ActorID:        actorID,
	})
	if err != nil {
		writeFormError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeForm(form), nil)
}

func (h *Handler) CreateVersion(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	formID, ok := parseUUID(r, "formId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid form ID")
		return
	}

	actorID := getUserID(r)
	if actorID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}

	version, err := h.svc.CreateVersion(r.Context(), application.CreateVersionParams{
		OrganizationID: orgID,
		FormID:         formID,
		ActorID:        actorID,
	})
	if err != nil {
		writeFormError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusCreated, serializeFormVersion(version), nil)
}

func (h *Handler) PublishVersion(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	versionID, ok := parseUUID(r, "versionId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid version ID")
		return
	}

	actorID := getUserID(r)
	if actorID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}

	version, err := h.svc.PublishVersion(r.Context(), application.PublishVersionParams{
		OrganizationID: orgID,
		VersionID:      versionID,
		ActorID:        actorID,
	})
	if err != nil {
		writeFormError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeFormVersion(version), nil)
}

func (h *Handler) ArchiveForm(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	formID, ok := parseUUID(r, "formId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid form ID")
		return
	}

	actorID := getUserID(r)
	if actorID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}

	form, err := h.svc.ArchiveForm(r.Context(), application.ArchiveFormParams{
		OrganizationID: orgID,
		FormID:         formID,
		ActorID:        actorID,
	})
	if err != nil {
		writeFormError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeForm(form), nil)
}

func (h *Handler) GetActiveVersion(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	formID, ok := parseUUID(r, "formId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid form ID")
		return
	}

	version, err := h.svc.GetActiveVersion(r.Context(), application.GetActiveVersionParams{
		OrganizationID: orgID,
		FormID:         formID,
	})
	if err != nil {
		writeFormError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeFormVersion(version), nil)
}

func (h *Handler) AddField(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	formID, ok := parseUUID(r, "formId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid form ID")
		return
	}

	actorID := getUserID(r)
	if actorID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}

	var req struct {
		VersionID    string              `json:"version_id"`
		Key          string              `json:"key"`
		Label        string              `json:"label"`
		Type         string              `json:"type"`
		Required     bool                `json:"required"`
		Description  string              `json:"description"`
		Placeholder  string              `json:"placeholder"`
		DefaultValue *string             `json:"default_value"`
		Validation   map[string]any      `json:"validation"`
		Options      []domain.FormOption `json:"options"`
		Order        int                 `json:"order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid request body")
		return
	}

	versionID, err := uuid.Parse(req.VersionID)
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid version ID")
		return
	}

	fieldType := domain.FieldType(req.Type)
	if !domain.IsValidFieldType(fieldType) {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid field type")
		return
	}

	field, err := h.svc.AddField(r.Context(), application.AddFieldParams{
		OrganizationID: orgID,
		FormID:         formID,
		VersionID:      versionID,
		Key:            req.Key,
		Label:          req.Label,
		Type:           fieldType,
		Required:       req.Required,
		Description:    req.Description,
		Placeholder:    req.Placeholder,
		DefaultValue:   req.DefaultValue,
		Validation:     req.Validation,
		Options:        req.Options,
		Order:          req.Order,
		ActorID:        actorID,
	})
	if err != nil {
		writeFormError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusCreated, serializeFormField(field), nil)
}

func (h *Handler) UpdateField(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	fieldID, ok := parseUUID(r, "fieldId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid field ID")
		return
	}

	actorID := getUserID(r)
	if actorID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}

	var req struct {
		Label        string              `json:"label"`
		Required     bool                `json:"required"`
		Description  string              `json:"description"`
		Placeholder  string              `json:"placeholder"`
		DefaultValue *string             `json:"default_value"`
		Validation   map[string]any      `json:"validation"`
		Options      []domain.FormOption `json:"options"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid request body")
		return
	}

	field, err := h.svc.UpdateField(r.Context(), application.UpdateFieldParams{
		OrganizationID: orgID,
		FieldID:        fieldID,
		Label:          req.Label,
		Required:       req.Required,
		Description:    req.Description,
		Placeholder:    req.Placeholder,
		DefaultValue:   req.DefaultValue,
		Validation:     req.Validation,
		Options:        req.Options,
		ActorID:        actorID,
	})
	if err != nil {
		writeFormError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeFormField(field), nil)
}

func (h *Handler) DeleteField(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	fieldID, ok := parseUUID(r, "fieldId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid field ID")
		return
	}

	actorID := getUserID(r)
	if actorID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}

	field, err := h.svc.DeleteField(r.Context(), application.DeleteFieldParams{
		OrganizationID: orgID,
		FieldID:        fieldID,
		ActorID:        actorID,
	})
	if err != nil {
		writeFormError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeFormField(field), nil)
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

func serializeForm(f *domain.Form) map[string]interface{} {
	result := map[string]interface{}{
		"id":              f.ID,
		"organization_id": f.OrganizationID,
		"key":             f.Key,
		"name":            f.Name,
		"description":     f.Description,
		"status":          f.Status,
		"created_by":      f.CreatedByID,
		"created_at":      f.CreatedAt,
		"updated_at":      f.UpdatedAt,
	}
	return result
}

func serializeFormVersion(v *domain.FormVersion) map[string]interface{} {
	result := map[string]interface{}{
		"id":              v.ID,
		"form_id":         v.FormID,
		"organization_id": v.OrganizationID,
		"version":         v.Version,
		"status":          v.Status,
		"created_by":      v.CreatedByID,
		"created_at":      v.CreatedAt,
		"updated_at":      v.UpdatedAt,
	}
	if v.PublishedAt != nil {
		result["published_at"] = v.PublishedAt
	} else {
		result["published_at"] = nil
	}
	return result
}

func serializeFormField(f *domain.FormField) map[string]interface{} {
	result := map[string]interface{}{
		"id":              f.ID,
		"form_id":         f.FormID,
		"form_version_id": f.FormVersionID,
		"organization_id": f.OrganizationID,
		"key":             f.Key,
		"label":           f.Label,
		"type":            f.Type,
		"required":        f.Required,
		"description":     f.Description,
		"placeholder":     f.Placeholder,
		"default_value":   f.DefaultValue,
		"validation":      f.Validation,
		"options":         f.Options,
		"order":           f.Order,
		"created_at":      f.CreatedAt,
		"updated_at":      f.UpdatedAt,
	}
	return result
}

func writeFormError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, application.ErrFormNotFound), errors.Is(err, domain.ErrFormNotFound), errors.Is(err, domain.ErrFormVersionNotFound), errors.Is(err, domain.ErrFormFieldNotFound):
		shared.WriteError(w, http.StatusNotFound, shared.CodeNotFound, "resource not found")
	case errors.Is(err, application.ErrFormInvalidInput), errors.Is(err, domain.ErrFormInvalidInput), errors.Is(err, domain.ErrFormFieldInvalid):
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, err.Error())
	case errors.Is(err, application.ErrFormKeyExists), errors.Is(err, domain.ErrFormKeyExists):
		shared.WriteError(w, http.StatusConflict, shared.CodeConflict, "form key already exists")
	case errors.Is(err, application.ErrFormInvalidStatus), errors.Is(err, domain.ErrFormInvalidStatus), errors.Is(err, application.ErrFormVersionStatus), errors.Is(err, domain.ErrFormVersionStatus):
		shared.WriteError(w, http.StatusConflict, shared.CodeConflict, err.Error())
	case errors.Is(err, application.ErrFormFieldTypeInvalid), errors.Is(err, domain.ErrFormFieldTypeInvalid):
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, err.Error())
	case errors.Is(err, application.ErrFormFieldDuplicate), errors.Is(err, domain.ErrFormFieldDuplicate):
		shared.WriteError(w, http.StatusConflict, shared.CodeConflict, "duplicate field key")
	case errors.Is(err, application.ErrFormOptionInvalid), errors.Is(err, domain.ErrFormOptionInvalid):
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, err.Error())
	default:
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "internal server error")
	}
}
