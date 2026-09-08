package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/alrazihi/civora/internal/identity/application"
	"github.com/alrazihi/civora/internal/identity/domain"
	"github.com/alrazihi/civora/internal/middleware"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	svc             IdentityService
	userRateLimiter *middleware.UserRateLimiter
}

type IdentityService interface {
	CreateUser(ctx context.Context, params application.CreateUserParams) (*domain.User, error)
	ListUsers(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.User, int, error)
	Authenticate(ctx context.Context, params application.AuthenticateParams) (*application.AuthenticateResult, error)
	GetUser(ctx context.Context, orgID, userID uuid.UUID) (*domain.User, error)
}

func NewHandler(svc IdentityService) *Handler {
	return &Handler{svc: svc}
}

func NewHandlerWithRateLimiter(svc IdentityService, userRateLimiter *middleware.UserRateLimiter) *Handler {
	return &Handler{svc: svc, userRateLimiter: userRateLimiter}
}

func (h *Handler) RegisterRoutes(r chi.Router, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/v1/organizations/{orgId}/auth", func(r chi.Router) {
		r.Post("/login", h.Login)
		r.Post("/register", h.Register)
	})
	r.Route("/api/v1/organizations/{orgId}/users", func(r chi.Router) {
		r.Use(authMiddleware)
		r.Use(middleware.RequireSameTenant)
		r.Use(middleware.RequireAnyRole("admin", "staff"))
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
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid request body")
		return
	}

	if h.userRateLimiter != nil && req.Email != "" {
		if locked, _, retryAfter := h.userRateLimiter.CheckRateLimit(req.Email); locked {
			retrySeconds := int(retryAfter.Seconds()) + 1
			body, err := json.Marshal(map[string]interface{}{
				"success": false,
				"error": map[string]string{
					"code":    "RATE_LIMITED",
					"message": fmt.Sprintf("Too many registration attempts for this email. Try again in %d seconds.", retrySeconds),
				},
			})
			if err != nil {
				log.Printf("marshal rate-limited response: %v", err)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", strconv.Itoa(retrySeconds))
			w.WriteHeader(http.StatusTooManyRequests)
			shared.WriteBody(w, body)
			return
		}
	}

	user, err := h.svc.CreateUser(r.Context(), application.CreateUserParams{
		OrganizationID: orgID,
		Email:          req.Email,
		Name:           req.Name,
		Password:       req.Password,
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
	if h.userRateLimiter != nil {
		var req struct {
			Email string `json:"email"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil && req.Email != "" {
			if locked, _, retryAfter := h.userRateLimiter.CheckRateLimit(req.Email); locked {
				retrySeconds := int(retryAfter.Seconds()) + 1
				body, err := json.Marshal(map[string]interface{}{
					"success": false,
					"error": map[string]string{
						"code":    "ACCOUNT_LOCKED",
						"message": fmt.Sprintf("Account temporarily locked due to too many failed login attempts. Try again in %d seconds.", retrySeconds),
					},
				})
				if err != nil {
					log.Printf("marshal account-locked response: %v", err)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", strconv.Itoa(retrySeconds))
				w.WriteHeader(http.StatusTooManyRequests)
				shared.WriteBody(w, body)
				return
			}
		}
	}

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
		if h.userRateLimiter != nil && req.Email != "" {
			h.userRateLimiter.RecordFailedAttempt(req.Email)
		}
		writeDomainError(w, err)
		return
	}

	if h.userRateLimiter != nil && req.Email != "" {
		h.userRateLimiter.RecordSuccessfulAttempt(req.Email)
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

	users, total, err := h.svc.ListUsers(r.Context(), mustParseUUID(orgID), perPage, offset)
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

	shared.WritePaginatedSuccess(w, http.StatusOK, result, page, perPage, total)
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
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "password must be at least 12 characters and contain at least one letter and one number")
	case application.ErrInvalidInput:
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid input")
	default:
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "internal server error")
	}
}
