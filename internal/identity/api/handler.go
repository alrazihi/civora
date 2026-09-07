package api

import (
	"encoding/json"
	"net/http"

	"github.com/alrazihi/civora/internal/identity/application"
	"github.com/alrazihi/civora/internal/middleware"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	svc *application.IdentityService
}

func NewHandler(svc *application.IdentityService) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(r chi.Router, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/v1/organizations/{orgId}/auth", func(r chi.Router) {
		r.Post("/login", h.Login)
		r.Post("/register", h.Register)
	})
	r.Route("/api/v1/organizations/{orgId}/users", func(r chi.Router) {
		r.Use(authMiddleware)
		r.Use(middleware.RequireSameTenant)
		r.Get("/", h.ListUsers)
		r.Get("/{userId}", h.GetUser)
	})
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseOrgID(r)
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	var req struct {
		Email    string `json:"email"`
		Name     string `json:"name"`
		Password string `json:"password"`
		Role     string `json:"role_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid request body")
		return
	}

	user, err := h.svc.CreateUser(r.Context(), application.CreateUserParams{
		OrganizationID: orgID,
		Email:          req.Email,
		Name:           req.Name,
		Password:       req.Password,
		RoleName:       req.Role,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusCreated, map[string]interface{}{
		"id":              user.ID,
		"organization_id": user.OrganizationID,
		"email":           user.Email,
		"name":            user.Name,
		"created_at":      user.CreatedAt,
	}, nil)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseOrgID(r)
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid request body")
		return
	}

	result, err := h.svc.Authenticate(r.Context(), application.AuthenticateParams{
		OrganizationID: orgID,
		Email:          req.Email,
		Password:       req.Password,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, map[string]interface{}{
		"token": result.Token,
		"user": map[string]interface{}{
			"id":              result.User.ID,
			"organization_id": result.User.OrganizationID,
			"email":           result.User.Email,
			"name":            result.User.Name,
		},
	}, nil)
}

func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	orgID := middleware.GetTenantID(r)
	users, err := h.svc.ListUsers(r.Context(), mustParseUUID(orgID))
	if err != nil {
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "failed to list users")
		return
	}

	result := make([]map[string]interface{}, len(users))
	for i, u := range users {
		result[i] = map[string]interface{}{
			"id":              u.ID,
			"organization_id": u.OrganizationID,
			"email":           u.Email,
			"name":            u.Name,
			"created_at":      u.CreatedAt,
		}
	}

	shared.WriteSuccess(w, http.StatusOK, result, nil)
}

func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	orgID := mustParseUUID(middleware.GetTenantID(r))
	userIDStr := chi.URLParam(r, "userId")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid user ID")
		return
	}

	user, err := h.svc.GetUser(r.Context(), orgID, userID)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, map[string]interface{}{
		"id":              user.ID,
		"organization_id": user.OrganizationID,
		"email":           user.Email,
		"name":            user.Name,
		"role_id":         user.RoleID,
		"created_at":      user.CreatedAt,
	}, nil)
}

func parseOrgID(r *http.Request) (uuid.UUID, bool) {
	orgIDStr := chi.URLParam(r, "orgId")
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		return uuid.Nil, false
	}
	return orgID, true
}

func mustParseUUID(s string) uuid.UUID {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil
	}
	return id
}

func writeDomainError(w http.ResponseWriter, err error) {
	switch err {
	case application.ErrInvalidCredentials:
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "invalid credentials")
	case application.ErrUserNotFound:
		shared.WriteError(w, http.StatusNotFound, shared.CodeNotFound, "user not found")
	case application.ErrEmailAlreadyExists:
		shared.WriteError(w, http.StatusConflict, shared.CodeConflict, "email already exists")
	case application.ErrInvalidEmail:
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid email format")
	case application.ErrWeakPassword:
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "password must be at least 8 characters and contain at least one letter and one number")
	case application.ErrInvalidInput:
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid input")
	default:
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "internal server error")
	}
}
