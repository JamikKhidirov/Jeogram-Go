package handler

import (
	"encoding/json"
	"net/http"

	"github.com/jeogram/messenger/internal/modules/user/domain"
	"github.com/jeogram/messenger/internal/modules/user/service"
	"github.com/jeogram/messenger/internal/pkg/middleware"
	"github.com/jeogram/messenger/internal/pkg/response"
	"github.com/jeogram/messenger/internal/pkg/validator"
)

type UserStatusHandler struct {
	svc *service.UserStatusService
}

func NewUserStatusHandler(svc *service.UserStatusService) *UserStatusHandler {
	return &UserStatusHandler{svc: svc}
}

// SetUserStatus устанавливает статус пользователя
// @Summary Установить статус
// @Tags user-status
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body domain.UpdateStatusRequest true "статус"
// @Success 200 {object} response.APIResponse
// @Router /user/status [post]
func (h *UserStatusHandler) SetUserStatus(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	var req domain.UpdateStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	if errs := validator.ValidateStruct(&req); len(errs) > 0 {
		response.WriteError(w, http.StatusUnprocessableEntity, "validation_error", firstErr(errs))
		return
	}

	if err := h.svc.UpdateStatus(r.Context(), userID, req.Status); err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	response.WriteOK(w, map[string]string{"status": string(req.Status)})
}

// GetUserStatus возвращает статус пользователя
// @Summary Получить статус
// @Tags user-status
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /user/status [get]
func (h *UserStatusHandler) GetUserStatus(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	status, err := h.svc.GetStatus(r.Context(), userID)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	response.WriteOK(w, map[string]string{"status": string(status)})
}

// UpdateUserSettings обновляет настройки пользователя
// @Summary Обновить настройки
// @Tags user-settings
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body domain.UpdateSettingsRequest true "настройки"
// @Success 200 {object} response.APIResponse
// @Router /user/settings [put]
func (h *UserStatusHandler) UpdateUserSettings(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	var req domain.UpdateSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}

	settings, err := h.svc.UpdateSettings(r.Context(), userID, req)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	response.WriteOK(w, settings)
}

// GetUserSettings возвращает настройки пользователя
// @Summary Получить настройки
// @Tags user-settings
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /user/settings [get]
func (h *UserStatusHandler) GetUserSettings(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	settings, err := h.svc.GetSettings(r.Context(), userID)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	response.WriteOK(w, settings)
}

// BlockUser блокирует пользователя
// @Summary Заблокировать пользователя
// @Tags user-blocking
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body domain.BlockUserRequest true "ID заблокированного"
// @Success 200 {object} response.APIResponse
// @Router /user/block [post]
func (h *UserStatusHandler) BlockUser(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	var req domain.BlockUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	if errs := validator.ValidateStruct(&req); len(errs) > 0 {
		response.WriteError(w, http.StatusUnprocessableEntity, "validation_error", firstErr(errs))
		return
	}

	if err := h.svc.BlockUser(r.Context(), userID, req.BlockedUserID, req.Reason); err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	response.WriteOK(w, map[string]string{"status": "blocked"})
}

// UnblockUser разблокирует пользователя
// @Summary Разблокировать пользователя
// @Tags user-blocking
// @Produce json
// @Security BearerAuth
// @Param blocked_user_id query string true "ID заблокированного"
// @Success 200 {object} response.APIResponse
// @Router /user/unblock [post]
func (h *UserStatusHandler) UnblockUser(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	blockedUserID := r.URL.Query().Get("blocked_user_id")
	if blockedUserID == "" {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "blocked_user_id is required")
		return
	}

	if err := h.svc.UnblockUser(r.Context(), userID, blockedUserID); err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	response.WriteOK(w, map[string]string{"status": "unblocked"})
}

// GetBlockedUsers возвращает список заблокированных пользователей
// @Summary Заблокированные пользователи
// @Tags user-blocking
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /user/blocked [get]
func (h *UserStatusHandler) GetBlockedUsers(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	users, err := h.svc.GetBlockedUsers(r.Context(), userID)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	response.WriteOK(w, users)
}

func firstErr(errs map[string]string) string {
	for _, v := range errs {
		return v
	}
	return "validation failed"
}
