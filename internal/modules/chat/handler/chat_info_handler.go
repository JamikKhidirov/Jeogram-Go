package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jeogram/messenger/internal/modules/chat/domain"
	"github.com/jeogram/messenger/internal/modules/chat/service"
	"github.com/jeogram/messenger/internal/pkg/middleware"
	"github.com/jeogram/messenger/internal/pkg/response"
	"github.com/jeogram/messenger/internal/pkg/validator"
)

type ChatInfoHandler struct {
	svc *service.ChatInfoService
}

func NewChatInfoHandler(svc *service.ChatInfoService) *ChatInfoHandler {
	return &ChatInfoHandler{svc: svc}
}

// GetChatInfo возвращает информацию о чате
// @Summary Информация о чате
// @Tags chat-info
// @Produce json
// @Security BearerAuth
// @Param chat_id path string true "id чата"
// @Success 200 {object} response.APIResponse
// @Router /chats/{chat_id}/info [get]
func (h *ChatInfoHandler) GetChatInfo(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	chatID := chi.URLParam(r, "chat_id")

	info, err := h.svc.GetChatInfo(r.Context(), chatID, userID)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	response.WriteOK(w, info)
}

// UpdateChatInfo обновляет информацию о чате
// @Summary Обновить информацию чата
// @Tags chat-info
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param chat_id path string true "id чата"
// @Param body body domain.UpdateChatInfoRequest true "данные чата"
// @Success 200 {object} response.APIResponse
// @Router /chats/{chat_id}/info [put]
func (h *ChatInfoHandler) UpdateChatInfo(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	chatID := chi.URLParam(r, "chat_id")
	var req domain.UpdateChatInfoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	if errs := validator.ValidateStruct(&req); len(errs) > 0 {
		response.WriteError(w, http.StatusUnprocessableEntity, "validation_error", firstErr(errs))
		return
	}

	info, err := h.svc.UpdateChatInfo(r.Context(), chatID, userID, req)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	response.WriteOK(w, info)
}

// GetChatStats возвращает статистику чата
// @Summary Статистика чата
// @Tags chat-info
// @Produce json
// @Security BearerAuth
// @Param chat_id path string true "id чата"
// @Success 200 {object} response.APIResponse
// @Router /chats/{chat_id}/stats [get]
func (h *ChatInfoHandler) GetChatStats(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	chatID := chi.URLParam(r, "chat_id")

	stats, err := h.svc.GetChatStats(r.Context(), chatID, userID)
	if err != nil {
		response.WriteError(w, http.StatusForbidden, "forbidden", err.Error())
		return
	}
	response.WriteOK(w, stats)
}

// LeaveChat удаляет пользователя из чата
// @Summary Выйти из чата
// @Tags chat-info
// @Produce json
// @Security BearerAuth
// @Param chat_id path string true "id чата"
// @Success 200 {object} response.APIResponse
// @Router /chats/{chat_id}/leave [post]
func (h *ChatInfoHandler) LeaveChat(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	chatID := chi.URLParam(r, "chat_id")

	if err := h.svc.LeaveChat(r.Context(), chatID, userID); err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	response.WriteOK(w, map[string]string{"status": "left"})
}

// DeleteChat удаляет чат (только владелец)
// @Summary Удалить чат
// @Tags chat-info
// @Produce json
// @Security BearerAuth
// @Param chat_id path string true "id чата"
// @Success 200 {object} response.APIResponse
// @Router /chats/{chat_id} [delete]
func (h *ChatInfoHandler) DeleteChat(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	chatID := chi.URLParam(r, "chat_id")

	if err := h.svc.DeleteChat(r.Context(), chatID, userID); err != nil {
		response.WriteError(w, http.StatusForbidden, "forbidden", err.Error())
		return
	}
	response.WriteOK(w, map[string]string{"status": "deleted"})
}

func firstErr(errs map[string]string) string {
	for _, v := range errs {
		return v
	}
	return "validation failed"
}
