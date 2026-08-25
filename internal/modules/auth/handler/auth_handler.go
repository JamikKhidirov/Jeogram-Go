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
	r.Post("/auth/request-otp", h.RequestOTP)
	r.Post("/auth/verify-otp", h.VerifyOTP)
	r.With(middleware.JWTAuth(h.jwt)).Post("/auth/logout", h.Logout)
	r.With(middleware.JWTAuth(h.jwt)).Post("/auth/logout-all", h.LogoutAll)
	r.With(middleware.JWTAuth(h.jwt)).Get("/auth/sessions", h.Sessions)
	r.With(middleware.JWTAuth(h.jwt)).Get("/auth/me", h.Me)
	r.With(middleware.JWTAuth(h.jwt)).Post("/auth/verify-email", h.VerifyEmail)
	r.With(middleware.JWTAuth(h.jwt)).Post("/auth/resend-verification", h.ResendVerification)
	r.Post("/auth/forgot-password", h.ForgotPassword)
	r.Post("/auth/reset-password", h.ResetPassword)
	r.With(middleware.JWTAuth(h.jwt)).Post("/auth/change-password", h.ChangePassword)
	r.With(middleware.JWTAuth(h.jwt)).Post("/auth/change-email/request", h.RequestEmailChange)
	r.With(middleware.JWTAuth(h.jwt)).Post("/auth/change-email", h.ChangeEmail)
	r.With(middleware.JWTAuth(h.jwt)).Post("/auth/change-phone/request", h.RequestPhoneChange)
	r.With(middleware.JWTAuth(h.jwt)).Post("/auth/change-phone", h.ChangePhone)
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

// LogoutAll выход из всех устройств (отзыв всех сессий).
// @Summary Выход со всех устройств
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /auth/logout-all [post]
func (h *AuthHandler) LogoutAll(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	if err := h.svc.LogoutAll(r.Context(), userID); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "internal_error", "could not logout all")
		return
	}
	response.WriteOK(w, map[string]string{"status": "logged_out_all"})
}

// Sessions возвращает историю входов пользователя (устройства, IP, время).
// @Summary История входов
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /auth/sessions [get]
func (h *AuthHandler) Sessions(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	tokens, err := h.svc.Sessions(r.Context(), userID)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	response.WriteOK(w, tokens)
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

// VerifyEmail подтверждает email по коду из письма.
// @Summary Подтверждение email кодом
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body verifyEmailRequest true "код"
// @Success 200 {object} response.APIResponse
// @Router /auth/verify-email [post]
func (h *AuthHandler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	var req verifyEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Code == "" {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "code required")
		return
	}
	if err := h.svc.VerifyEmail(r.Context(), userID, req.Code); err != nil {
		handleAuthError(w, err)
		return
	}
	response.WriteOK(w, map[string]string{"status": "email_verified"})
}

type verifyEmailRequest struct {
	Code string `json:"code"`
}

// ResendVerification повторно отправляет код подтверждения email.
// @Summary Повторная отправка кода подтверждения email
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /auth/resend-verification [post]
func (h *AuthHandler) ResendVerification(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	if err := h.svc.ResendVerification(r.Context(), userID); err != nil {
		handleAuthError(w, err)
		return
	}
	response.WriteOK(w, map[string]string{"status": "code_sent"})
}

type otpRequest struct {
	Phone string `json:"phone"`
}

// RequestOTP запрашивает OTP-код для входа по номеру телефона.
// @Summary Запросить OTP-код (вход по телефону)
// @Tags auth
// @Accept json
// @Produce json
// @Param body body otpRequest true "номер телефона"
// @Success 200 {object} response.APIResponse
// @Router /auth/request-otp [post]
func (h *AuthHandler) RequestOTP(w http.ResponseWriter, r *http.Request) {
	var req otpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Phone == "" {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "phone required")
		return
	}
	if err := h.svc.RequestPhoneOTP(r.Context(), req.Phone); err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	response.WriteOK(w, map[string]string{"status": "otp_sent", "phone": req.Phone})
}

type verifyOTPRequest struct {
	Phone string `json:"phone"`
	Code  string `json:"code"`
}

// VerifyOTP выполняет вход по номеру телефона и OTP-коду.
// @Summary Вход по телефону и OTP-коду
// @Tags auth
// @Accept json
// @Produce json
// @Param body body verifyOTPRequest true "номер и код"
// @Success 200 {object} response.APIResponse
// @Router /auth/verify-otp [post]
func (h *AuthHandler) VerifyOTP(w http.ResponseWriter, r *http.Request) {
	var req verifyOTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Phone == "" || req.Code == "" {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "phone and code required")
		return
	}
	res, err := h.svc.LoginPhone(r.Context(), req.Phone, req.Code)
	if err != nil {
		handleAuthError(w, err)
		return
	}
	response.WriteOK(w, res)
}

type forgotPasswordRequest struct {
	Email string `json:"email"`
}

// ForgotPassword отправляет код сброса пароля на email.
// @Summary Запросить сброс пароля (код на email)
// @Tags auth
// @Accept json
// @Produce json
// @Param body body forgotPasswordRequest true "email"
// @Success 200 {object} response.APIResponse
// @Router /auth/forgot-password [post]
func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req forgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "email required")
		return
	}
	_ = h.svc.ForgotPassword(r.Context(), req.Email)
	// Всегда возвращаем успех, чтобы не раскрывать наличие аккаунта.
	response.WriteOK(w, map[string]string{"status": "if_email_exists_code_sent"})
}

type resetPasswordRequest struct {
	Email    string `json:"email"`
	Code     string `json:"code"`
	Password string `json:"password"`
}

// ResetPassword меняет пароль по email + коду.
// @Summary Сброс пароля по коду
// @Tags auth
// @Accept json
// @Produce json
// @Param body body resetPasswordRequest true "email, code, новый пароль"
// @Success 200 {object} response.APIResponse
// @Router /auth/reset-password [post]
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req resetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" || req.Code == "" || req.Password == "" {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "email, code and password required")
		return
	}
	if err := h.svc.ResetPassword(r.Context(), req.Email, req.Code, req.Password); err != nil {
		handleAuthError(w, err)
		return
	}
	response.WriteOK(w, map[string]string{"status": "password_reset"})
}

type changePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// ChangePassword меняет пароль при известном старом.
// @Summary Сменить пароль
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body changePasswordRequest true "old_password, new_password"
// @Success 200 {object} response.APIResponse
// @Router /auth/change-password [post]
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	var req changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.OldPassword == "" || req.NewPassword == "" {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "old_password and new_password required")
		return
	}
	if err := h.svc.ChangePassword(r.Context(), userID, req.OldPassword, req.NewPassword); err != nil {
		handleAuthError(w, err)
		return
	}
	response.WriteOK(w, map[string]string{"status": "password_changed"})
}

type changeEmailRequest struct {
	NewEmail string `json:"new_email"`
	Code     string `json:"code"`
}

// RequestEmailChange запрашивает смену email (код приходит на новый email).
// @Summary Запрос смены email
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body changeEmailRequest true "new_email"
// @Success 200 {object} response.APIResponse
// @Router /auth/change-email/request [post]
func (h *AuthHandler) RequestEmailChange(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	var req changeEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.NewEmail == "" {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "new_email required")
		return
	}
	if err := h.svc.RequestEmailChange(r.Context(), userID, req.NewEmail); err != nil {
		handleAuthError(w, err)
		return
	}
	response.WriteOK(w, map[string]string{"status": "code_sent"})
}

// ChangeEmail подтверждает смену email кодом.
// @Summary Подтвердить смену email
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body changeEmailRequest true "new_email, code"
// @Success 200 {object} response.APIResponse
// @Router /auth/change-email [post]
func (h *AuthHandler) ChangeEmail(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	var req changeEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.NewEmail == "" || req.Code == "" {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "new_email and code required")
		return
	}
	if err := h.svc.ChangeEmail(r.Context(), userID, req.NewEmail, req.Code); err != nil {
		handleAuthError(w, err)
		return
	}
	response.WriteOK(w, map[string]string{"status": "email_changed"})
}

type changePhoneRequest struct {
	NewPhone string `json:"new_phone"`
	Code     string `json:"code"`
}

// RequestPhoneChange запрашивает смену телефона (OTP приходит на email/лог).
// @Summary Запрос смены телефона
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body changePhoneRequest true "new_phone"
// @Success 200 {object} response.APIResponse
// @Router /auth/change-phone/request [post]
func (h *AuthHandler) RequestPhoneChange(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	var req changePhoneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.NewPhone == "" {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "new_phone required")
		return
	}
	if err := h.svc.RequestPhoneChange(r.Context(), userID, req.NewPhone); err != nil {
		handleAuthError(w, err)
		return
	}
	response.WriteOK(w, map[string]string{"status": "code_sent"})
}

// ChangePhone подтверждает смену телефона кодом.
// @Summary Подтвердить смену телефона
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body changePhoneRequest true "new_phone, code"
// @Success 200 {object} response.APIResponse
// @Router /auth/change-phone [post]
func (h *AuthHandler) ChangePhone(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	var req changePhoneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.NewPhone == "" || req.Code == "" {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "new_phone and code required")
		return
	}
	if err := h.svc.ChangePhone(r.Context(), userID, req.NewPhone, req.Code); err != nil {
		handleAuthError(w, err)
		return
	}
	response.WriteOK(w, map[string]string{"status": "phone_changed"})
}

func handleAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidCredentials):
		response.WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid credentials")
	case errors.Is(err, service.ErrEmailNotVerified):
		response.WriteError(w, http.StatusForbidden, "forbidden", "email not verified")
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
