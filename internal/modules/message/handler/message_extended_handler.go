package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jeogram/messenger/internal/modules/message/domain"
	"github.com/jeogram/messenger/internal/modules/message/service"
	"github.com/jeogram/messenger/internal/pkg/middleware"
	"github.com/jeogram/messenger/internal/pkg/response"
	"github.com/jeogram/messenger/internal/pkg/validator"
)

type MessageExtendedHandler struct {
	svc *service.MessageExtendedService
}

func NewMessageExtendedHandler(svc *service.MessageExtendedService) *MessageExtendedHandler {
	return &MessageExtendedHandler{svc: svc}
}

// QuoteMessage отвечает на сообщение (reply/quote)
// @Summary Ответить на сообщение
// @Tags messages-extended
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param message_id path string true "ID сообщения для ответа"
// @Param body body domain.QuoteRequest true "текст ответа"
// @Success 201 {object} response.APIResponse
// @Router /messages/{message_id}/reply [post]
func (h *MessageExtendedHandler) QuoteMessage(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	messageID := chi.URLParam(r, "message_id")

	var req domain.QuoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}

	if errs := validator.ValidateStruct(&req); len(errs) > 0 {
		response.WriteError(w, http.StatusUnprocessableEntity, "validation_error", getFirstErr(errs))
		return
	}

	msg, err := h.svc.Quote(r.Context(), userID, messageID, req.Text)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	response.WriteCreated(w, msg)
}

// ListUnsent получает неотправленные сообщения
// @Summary Неотправленные сообщения
// @Tags messages-extended
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /messages/unsent [get]
func (h *MessageExtendedHandler) ListUnsent(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)

	messages, err := h.svc.ListUnsent(r.Context(), userID)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	response.WriteOK(w, messages)
}

func getFirstErr(errs map[string]string) string {
	for _, v := range errs {
		return v
	}
	return "validation failed"
}
