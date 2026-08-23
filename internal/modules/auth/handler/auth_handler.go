package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jeogram/messenger/internal/modules/auth/domain"
	"github.com/jeogram/messenger/internal/modules/auth/repository"
	"github.com/jeogram/messenger/internal/modules/auth/service"
	"github.com/jeogram/messenger/internal/pkg/auth"
	"github.com/jeogram/messenger/internal/pkg/middleware"
	"github.com/jeogram/messenger/internal/pkg/response"
	"github.com/jeogram/messenger/internal/pkg/validator"
)

// AuthHandler exposes HTTP endpoints for authentication.
type AuthHandler struct {
	svc *service.AuthService
	jwt *auth.JWT
}

func NewAuthHandler(svc *service.AuthService, jwt *auth.JWT) *AuthHandler {
	return &AuthHandler{svc: svc, jwt: jwt}
}

// RegisterRoutes mounts auth endpoints on the given router.
//
//	@Summary	Register a new account
//	@Tags		auth
//	@Accept		json
//	@Produce	json
//	@Param		body	body	domain.RegisterRequest	true	"registration payload"
//	@Success	201	{object}	response.APIResponse
//	@Router		/auth/register [post]
func (h *AuthHandler) RegisterRoutes(r chi.Router) {
	r.Post("/auth/register", h.Register)
	r.Post("/auth/login", h.Login)
	r.Post("/auth/refresh", h.Refresh)
	r.With(middleware.JWTAuth(h.jwt)).Post("/auth/logout", h.Logout)
	r.With(middleware.JWTAuth(h.jwt)).Get("/auth/me", h.Me)
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req domain.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "invalid json body")
		return
	}
	if errs := validator.ValidateStruct(&req); len(errs) > 0 {
		response.WriteError(w, http.StatusUnprocessableEntity, "validation_error", firstErr(errs))
		return
	}
	res, err := h.svc.Register(r.Context(), req)
	if err != nil {
		handleAuthError(w, err)
		return
	}
	response.WriteCreated(w, res)
}

// Login аутентифицирует пользователя по email и паролю.
// @Summary Вход в систему
// @Tags auth
// @Accept json
// @Produce json
// @Param body body domain.LoginRequest true "учётные данные"
// @Success 200 {object} response.APIResponse
// @Router /auth/login [post]
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req domain.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "invalid json body")
		return
	}
	if errs := validator.ValidateStruct(&req); len(errs) > 0 {
		response.WriteError(w, http.StatusUnprocessableEntity, "validation_error", firstErr(errs))
		return
	}
	res, err := h.svc.Login(r.Context(), req)
	if err != nil {
		handleAuthError(w, err)
		return
	}
	response.WriteOK(w, res)
}

// Refresh обновляет пару токенов по refresh-токену.
// @Summary Обновление токена
// @Tags auth
// @Accept json
// @Produce json
// @Param body body domain.RefreshRequest true "refresh-токен"
// @Success 200 {object} response.APIResponse
// @Router /auth/refresh [post]
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req domain.RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "invalid json body")
		return
	}
	res, err := h.svc.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		handleAuthError(w, err)
		return
	}
	response.WriteOK(w, res)
}

// Logout выход из системы (отзыв refresh-токена).
// @Summary Выход
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /auth/logout [post]
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	if err := h.svc.Logout(r.Context(), userID); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "internal_error", "could not logout")
		return
	}
	response.WriteOK(w, map[string]string{"status": "logged_out"})
}

// Me возвращает профиль текущего пользователя.
// @Summary Профиль текущего пользователя
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /auth/me [get]
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	u, err := h.svc.Me(r.Context(), userID)
	if err != nil {
		handleAuthError(w, err)
		return
	}
	response.WriteOK(w, u)
}

func handleAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidCredentials):
		response.WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid credentials")
	case errors.Is(err, repository.ErrUserNotFound):
		response.WriteError(w, http.StatusNotFound, "not_found", "user not found")
	case errors.Is(err, repository.ErrDuplicateEmail):
		response.WriteError(w, http.StatusConflict, "conflict", "email already in use")
	case errors.Is(err, repository.ErrDuplicateUsername):
		response.WriteError(w, http.StatusConflict, "conflict", "username already in use")
	default:
		response.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
	}
}

func firstErr(errs map[string]string) string {
	for _, v := range errs {
		return v
	}
	return "validation failed"
}
