package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jeogram/messenger/internal/modules/notification/domain"
	"github.com/jeogram/messenger/internal/modules/notification/service"
	"github.com/jeogram/messenger/internal/pkg/auth"
	"github.com/jeogram/messenger/internal/pkg/middleware"
	"github.com/jeogram/messenger/internal/pkg/response"
	"github.com/jeogram/messenger/internal/pkg/validator"
)

// NotificationHandler exposes notification endpoints.
type NotificationHandler struct {
	svc *service.NotificationService
	jwt *auth.JWT
}

func NewNotificationHandler(svc *service.NotificationService, jwt *auth.JWT) *NotificationHandler {
	return &NotificationHandler{svc: svc, jwt: jwt}
}

// RegisterRoutes mounts notification endpoints.
//
//	@Summary	Register a push device token
//	@Tags		notifications
//	@Accept		json
//	@Produce	json
//	@Param		body	body	domain.RegisterDeviceRequest	true	"device token"
//	@Success	201	{object}	response.APIResponse
//	@Router		/notifications/device [post]
//	@Security	BearerAuth
func (h *NotificationHandler) RegisterRoutes(r chi.Router) {
	r.With(middleware.JWTAuth(h.jwt)).Post("/notifications/device", h.RegisterDevice)
	r.With(middleware.JWTAuth(h.jwt)).Get("/notifications", h.List)
	r.With(middleware.JWTAuth(h.jwt)).Post("/notifications/read", h.MarkRead)
}

// RegisterDevice регистрирует push-токен устройства.
// @Summary Регистрация устройства (push)
// @Tags notifications
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body domain.RegisterDeviceRequest true "платформа и токен"
// @Success 201 {object} response.APIResponse
// @Router /notifications/device [post]
func (h *NotificationHandler) RegisterDevice(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	var req domain.RegisterDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	if errs := validator.ValidateStruct(&req); len(errs) > 0 {
		response.WriteError(w, http.StatusUnprocessableEntity, "validation_error", firstErr(errs))
		return
	}
	h.svc.RegisterDevice(r.Context(), userID, req.Platform, req.Token)
	response.WriteCreated(w, map[string]string{"status": "registered"})
}

// List возвращает ленту уведомлений пользователя.
// @Summary Лента уведомлений
// @Tags notifications
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /notifications [get]
func (h *NotificationHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	ns, err := h.svc.List(r.Context(), userID)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	response.WriteOK(w, ns)
}

// MarkRead помечает все уведомления прочитанными.
// @Summary Прочитать уведомления
// @Tags notifications
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /notifications/read [post]
func (h *NotificationHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	if err := h.svc.MarkAllRead(r.Context(), userID); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	response.WriteOK(w, map[string]string{"status": "read"})
}

func firstErr(errs map[string]string) string {
	for _, v := range errs {
		return v
	}
	return "validation failed"
}
