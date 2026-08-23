package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jeogram/messenger/internal/modules/chat/domain"
	"github.com/jeogram/messenger/internal/pkg/middleware"
	"github.com/jeogram/messenger/internal/pkg/response"
	"github.com/jeogram/messenger/internal/pkg/validator"
)

// UpdateAvatar обновляет аватар чата.
// @Summary Обновить аватар чата
// @Tags chats
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param chat_id path string true "id чата"
// @Param body body domain.UpdateAvatarRequest true "URL аватара"
// @Success 200 {object} response.APIResponse
// @Router /chats/{chat_id}/avatar [put]
func (h *ChatHandler) UpdateAvatar(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	chatID := chi.URLParam(r, "chat_id")
	var req domain.UpdateAvatarRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	chat, err := h.svc.UpdateAvatar(r.Context(), chatID, userID, req.AvatarURL)
	if err != nil {
		writeChatError(w, err)
		return
	}
	response.WriteOK(w, chat)
}

// MuteChat отключает уведомления для чата.
// @Summary Отключить уведомления (чат)
// @Tags chats
// @Produce json
// @Security BearerAuth
// @Param chat_id path string true "id чата"
// @Param duration query string false "длительность (1h, 8h, 24h, permanent)"
// @Success 200 {object} response.APIResponse
// @Router /chats/{chat_id}/mute [post]
func (h *ChatHandler) MuteChat(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	chatID := chi.URLParam(r, "chat_id")
	duration := r.URL.Query().Get("duration")
	if duration == "" {
		duration = "permanent"
	}
	if err := h.svc.Mute(r.Context(), chatID, userID, duration); err != nil {
		writeChatError(w, err)
		return
	}
	response.WriteOK(w, map[string]string{"status": "muted"})
}

// UnmuteChat включает уведомления для чата.
// @Summary Включить уведомления (чат)
// @Tags chats
// @Produce json
// @Security BearerAuth
// @Param chat_id path string true "id чата"
// @Success 200 {object} response.APIResponse
// @Router /chats/{chat_id}/mute [delete]
func (h *ChatHandler) UnmuteChat(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	chatID := chi.URLParam(r, "chat_id")
	if err := h.svc.Unmute(r.Context(), chatID, userID); err != nil {
		writeChatError(w, err)
		return
	}
	response.WriteOK(w, map[string]string{"status": "unmuted"})
}

// ListMembers возвращает членов чата с пагинацией.
// @Summary Члены чата
// @Tags chats
// @Produce json
// @Security BearerAuth
// @Param chat_id path string true "id чата"
// @Param limit query int false "лимит (по умолчанию 50)"
// @Param offset query int false "смещение"
// @Success 200 {object} response.APIResponse
// @Router /chats/{chat_id}/members [get]
func (h *ChatHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	chatID := chi.URLParam(r, "chat_id")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	members, err := h.svc.ListMembers(r.Context(), chatID, userID, limit, offset)
	if err != nil {
		writeChatError(w, err)
		return
	}
	response.WriteOK(w, members)
}

// SetChatDescription устанавливает описание чата.
// @Summary Установить описание чата
// @Tags chats
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param chat_id path string true "id чата"
// @Param body body domain.UpdateDescriptionRequest true "новое описание"
// @Success 200 {object} response.APIResponse
// @Router /chats/{chat_id}/description [put]
func (h *ChatHandler) SetChatDescription(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	chatID := chi.URLParam(r, "chat_id")
	var req domain.UpdateDescriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	if errs := validator.ValidateStruct(&req); len(errs) > 0 {
		response.WriteError(w, http.StatusUnprocessableEntity, "validation_error", firstErr(errs))
		return
	}
	chat, err := h.svc.UpdateDescription(r.Context(), chatID, userID, req.Description)
	if err != nil {
		writeChatError(w, err)
		return
	}
	response.WriteOK(w, chat)
}

// RegisterExtraRoutes регистрирует дополнительные эндпоинты чата.
func (h *ChatHandler) RegisterExtraRoutes(r chi.Router) {
	// Аватар чата
	r.With(middleware.JWTAuth(h.jwt)).Put("/chats/{chat_id}/avatar", h.UpdateAvatar)

	// Уведомления
	r.With(middleware.JWTAuth(h.jwt)).Post("/chats/{chat_id}/mute", h.MuteChat)
	r.With(middleware.JWTAuth(h.jwt)).Delete("/chats/{chat_id}/mute", h.UnmuteChat)

	// Члены чата
	r.With(middleware.JWTAuth(h.jwt)).Get("/chats/{chat_id}/members", h.ListMembers)

	// Описание
	r.With(middleware.JWTAuth(h.jwt)).Put("/chats/{chat_id}/description", h.SetChatDescription)
}

func firstErr(errs map[string]string) string {
	for _, v := range errs {
		return v
	}
	return "validation failed"
}
