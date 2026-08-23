package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jeogram/messenger/internal/modules/message/domain"
	"github.com/jeogram/messenger/internal/pkg/middleware"
	"github.com/jeogram/messenger/internal/pkg/response"
	"github.com/jeogram/messenger/internal/pkg/validator"
)

// Quote цитирует сообщение (reply).
// @Summary Цитировать сообщение (reply)
// @Tags messages
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "id сообщения для ответа"
// @Param body body domain.QuoteRequest true "текст ответа"
// @Success 201 {object} response.APIResponse
// @Router /messages/{id}/reply [post]
func (h *MessageHandler) Quote(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	id := chi.URLParam(r, "id")
	var req domain.QuoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	if errs := validator.ValidateStruct(&req); len(errs) > 0 {
		response.WriteError(w, http.StatusUnprocessableEntity, "validation_error", firstErr(errs))
		return
	}
	msg, err := h.svc.Quote(r.Context(), userID, id, req.Text)
	if err != nil {
		writeMsgError(w, err)
		return
	}
	response.WriteCreated(w, msg)
}

// Edit2 редактирует сообщение с указанием источника.
func (h *MessageHandler) Edit2(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	id := chi.URLParam(r, "id")
	var req domain.EditMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	msg, err := h.svc.Edit(r.Context(), id, userID, req.Text)
	if err != nil {
		writeMsgError(w, err)
		return
	}
	response.WriteOK(w, msg)
}

// ListUnsent возвращает неотправленные сообщения текущего пользователя.
// @Summary Неотправленные сообщения
// @Tags messages
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /messages/unsent [get]
func (h *MessageHandler) ListUnsent(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	messages, err := h.svc.ListUnsent(r.Context(), userID)
	if err != nil {
		writeMsgError(w, err)
		return
	}
	response.WriteOK(w, messages)
}

// RegisterExtraRoutes регистрирует дополнительные эндпоинты.
func (h *MessageHandler) RegisterExtraRoutes(r chi.Router) {
	// Цитирование / ответы
	r.With(middleware.JWTAuth(h.jwt)).Post("/messages/{id}/reply", h.Quote)

	// Неотправленные сообщения
	r.With(middleware.JWTAuth(h.jwt)).Get("/messages/unsent", h.ListUnsent)
}
