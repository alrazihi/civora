package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/alrazihi/civora/internal/middleware"
	"github.com/alrazihi/civora/internal/people/application"
	"github.com/alrazihi/civora/internal/people/domain"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	svc PersonService
}

func NewHandler(svc PersonService) *Handler {
	return &Handler{svc: svc}
}

type PersonService interface {
	CreatePerson(ctx context.Context, params application.CreatePersonParams) (*domain.Person, error)
	GetPerson(ctx context.Context, orgID, id uuid.UUID) (*domain.Person, error)
	ListPersons(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.Person, int, error)
	FindPersonByExternalReference(ctx context.Context, orgID uuid.UUID, ref string) (*domain.Person, error)
	UpdatePersonStatus(ctx context.Context, orgID, id uuid.UUID, status domain.PersonStatus, actorID uuid.UUID) (*domain.Person, error)
}

func (h *Handler) RegisterRoutes(r chi.Router, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/v1/organizations/{orgId}/people", func(r chi.Router) {
		r.Use(authMiddleware)
		r.Use(middleware.RequireSameTenant)
		r.Post("/", h.CreatePerson)
		r.Get("/", h.ListPersons)
		r.Get("/{personId}", h.GetPerson)
		r.Get("/external/{externalRef}", h.FindByExternalReference)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAnyRole("admin", "staff"))
			r.Patch("/{personId}/status", h.UpdatePersonStatus)
		})
	})
}

func (h *Handler) CreatePerson(w http.ResponseWriter, r *http.Request) {
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
		ExternalReference *string `json:"external_reference"`
		FirstName         string  `json:"first_name"`
		LastName          string  `json:"last_name"`
		DateOfBirth       *string `json:"date_of_birth"`
		Email             *string `json:"email"`
		Phone             *string `json:"phone"`
		Address           *string `json:"address"`
		City              *string `json:"city"`
		PreferredLanguage string  `json:"preferred_language"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid request body")
		return
	}

	var dob *time.Time
	if req.DateOfBirth != nil && *req.DateOfBirth != "" {
		parsed, err := time.Parse("2006-01-02", *req.DateOfBirth)
		if err != nil {
			shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid date_of_birth format, use YYYY-MM-DD")
			return
		}
		dob = &parsed
	}

	p, err := h.svc.CreatePerson(r.Context(), application.CreatePersonParams{
		OrganizationID:    orgID,
		ExternalReference: req.ExternalReference,
		FirstName:         req.FirstName,
		LastName:          req.LastName,
		DateOfBirth:       dob,
		Email:             req.Email,
		Phone:             req.Phone,
		Address:           req.Address,
		City:              req.City,
		PreferredLanguage: req.PreferredLanguage,
		ActorID:           actorID,
	})
	if err != nil {
		writePersonError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusCreated, serializePerson(p), nil)
}

func (h *Handler) GetPerson(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	personID, ok := parseUUID(r, "personId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid person ID")
		return
	}

	p, err := h.svc.GetPerson(r.Context(), orgID, personID)
	if err != nil {
		writePersonError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializePerson(p), nil)
}

func (h *Handler) ListPersons(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
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

	persons, total, err := h.svc.ListPersons(r.Context(), orgID, perPage, offset)
	if err != nil {
		writePersonError(w, err)
		return
	}

	result := make([]map[string]interface{}, len(persons))
	for i, p := range persons {
		result[i] = serializePerson(p)
	}

	shared.WritePaginatedSuccess(w, http.StatusOK, result, page, perPage, total)
}

func (h *Handler) FindByExternalReference(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	externalRef := chi.URLParam(r, "externalRef")
	if externalRef == "" {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "external reference is required")
		return
	}

	p, err := h.svc.FindPersonByExternalReference(r.Context(), orgID, externalRef)
	if err != nil {
		writePersonError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializePerson(p), nil)
}

func (h *Handler) UpdatePersonStatus(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	personID, ok := parseUUID(r, "personId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid person ID")
		return
	}

	actorID := getUserID(r)
	if actorID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid request body")
		return
	}

	p, err := h.svc.UpdatePersonStatus(r.Context(), orgID, personID, domain.PersonStatus(req.Status), actorID)
	if err != nil {
		writePersonError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializePerson(p), nil)
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

func serializePerson(p *domain.Person) map[string]interface{} {
	result := map[string]interface{}{
		"id":                 p.ID,
		"organization_id":    p.OrganizationID,
		"first_name":         p.FirstName,
		"last_name":          p.LastName,
		"preferred_language": p.PreferredLanguage,
		"status":             p.Status,
		"created_at":         p.CreatedAt,
		"updated_at":         p.UpdatedAt,
	}
	if p.ExternalReference != nil {
		result["external_reference"] = p.ExternalReference
	}
	if p.DateOfBirth != nil {
		result["date_of_birth"] = p.DateOfBirth
	}
	if p.Email != nil {
		result["email"] = p.Email
	}
	if p.Phone != nil {
		result["phone"] = p.Phone
	}
	if p.Address != nil {
		result["address"] = p.Address
	}
	if p.City != nil {
		result["city"] = p.City
	}
	return result
}

func writePersonError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, application.ErrPersonNotFound), errors.Is(err, domain.ErrPersonNotFound):
		shared.WriteError(w, http.StatusNotFound, shared.CodeNotFound, "person not found")
	case errors.Is(err, application.ErrPersonInvalidInput), errors.Is(err, domain.ErrPersonInvalidInput):
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid input")
	case errors.Is(err, application.ErrPersonTenantViolation):
		shared.WriteError(w, http.StatusForbidden, shared.CodeTenantViolation, "tenant violation")
	default:
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "internal server error")
	}
}
