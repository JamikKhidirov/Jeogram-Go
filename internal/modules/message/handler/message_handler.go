package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jeogram/messenger/internal/modules/message/domain"
	msgrepo "github.com/jeogram/messenger/internal/modules/message/repository"
	"github.com/jeogram/messenger/internal/modules/message/service"
	"github.com/jeogram/messenger/internal/pkg/auth"
	"github.com/jeogram/messenger/internal/pkg/middleware"
	"github.com/jeogram/messenger/internal/pkg/response"
	"github.com/jeogram/messenger/internal/pkg/validator"
)

// MessageHandler exposes message endpoints.
type MessageHandler struct {
	svc *service.MessageService
	jwt *auth.JWT
}

func NewMessageHandler(svc *service.MessageService, jwt *auth.JWT) *MessageHandler {
	return &MessageHandler{svc: svc, jwt: jwt}
}

// RegisterRoutes mounts message endpoints.
//
//	@Summary	Send a message (text/voice/image)
//	@Tags		messages
//	@Accept		json
//	@Produce	json
//	@Param		body	body	domain.SendMessageRequest	true	"message payload"
//	@Success	201	{object}	response.APIResponse
//	@Router		/messages [post]
//	@Security	BearerAuth
func (h *MessageHandler) RegisterRoutes(r chi.Router) {
	r.With(middleware.JWTAuth(h.jwt)).Post("/messages", h.Send)
	r.With(middleware.JWTAuth(h.jwt)).Get("/chats/{chat_id}/messages", h.List)
	r.With(middleware.JWTAuth(h.jwt)).Put("/messages/{id}", h.Edit)
	r.With(middleware.JWTAuth(h.jwt)).Delete("/messages/{id}", h.Delete)
	r.With(middleware.JWTAuth(h.jwt)).Post("/chats/{chat_id}/read", h.MarkRead)
	r.With(middleware.JWTAuth(h.jwt)).Get("/chats/{chat_id}/unread", h.Unread)

	// Реакции.
	r.With(middleware.JWTAuth(h.jwt)).Post("/messages/{id}/reactions", h.React)
	r.With(middleware.JWTAuth(h.jwt)).Delete("/messages/{id}/reactions", h.Unreact)
	r.With(middleware.JWTAuth(h.jwt)).Get("/messages/{id}/reactions", h.ListReactions)

	// Закрепление.
	r.With(middleware.JWTAuth(h.jwt)).Post("/chats/{chat_id}/pin/{message_id}", h.Pin)
	r.With(middleware.JWTAuth(h.jwt)).Delete("/chats/{chat_id}/pin/{message_id}", h.Unpin)

	// Пересылка.
	r.With(middleware.JWTAuth(h.jwt)).Post("/messages/{id}/forward", h.Forward)

	// Поиск сообщений.
	r.With(middleware.JWTAuth(h.jwt)).Get("/chats/{chat_id}/messages/search", h.SearchInChat)
	r.With(middleware.JWTAuth(h.jwt)).Get("/messages/search", h.SearchGlobal)

	// Индикатор печати.
	r.With(middleware.JWTAuth(h.jwt)).Post("/chats/{chat_id}/typing", h.Typing)

	// Удаление сообщения для всех (admin/owner чата).
	r.With(middleware.JWTAuth(h.jwt)).Delete("/chats/{chat_id}/messages/{id}/admin", h.DeleteForAll)
}

// Send отправляет сообщение (текст/голос/картинка) в чат.
// @Summary Отправить сообщение
// @Tags messages
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body domain.SendMessageRequest true "message payload"
// @Success 201 {object} response.APIResponse
// @Router /messages [post]
func (h *MessageHandler) Send(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	var req domain.SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	if errs := validator.ValidateStruct(&req); len(errs) > 0 {
		response.WriteError(w, http.StatusUnprocessableEntity, "validation_error", firstErr(errs))
		return
	}
	msg, err := h.svc.Send(r.Context(), userID, req)
	if err != nil {
		writeMsgError(w, err)
		return
	}
	response.WriteCreated(w, msg)
}

// List возвращает сообщения чата с пагинацией.
// @Summary Список сообщений чата
// @Tags messages
// @Produce json
// @Security BearerAuth
// @Param chat_id path string true "id чата"
// @Param limit query int false "лимит (по умолчанию 50)"
// @Param offset query int false "смещение"
// @Success 200 {object} response.APIResponse
// @Router /chats/{chat_id}/messages [get]
func (h *MessageHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	chatID := chi.URLParam(r, "chat_id")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	msgs, err := h.svc.List(r.Context(), chatID, userID, limit, offset)
	if err != nil {
		writeMsgError(w, err)
		return
	}
	response.WriteOK(w, msgs)
}

// Edit редактирует своё сообщение.
// @Summary Редактировать сообщение
// @Tags messages
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "id сообщения"
// @Param body body domain.EditMessageRequest true "новый текст"
// @Success 200 {object} response.APIResponse
// @Router /messages/{id} [put]
func (h *MessageHandler) Edit(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	id := chi.URLParam(r, "id")
	var req domain.EditMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	if errs := validator.ValidateStruct(&req); len(errs) > 0 {
		response.WriteError(w, http.StatusUnprocessableEntity, "validation_error", firstErr(errs))
		return
	}
	msg, err := h.svc.Edit(r.Context(), id, userID, req.Text)
	if err != nil {
		writeMsgError(w, err)
		return
	}
	response.WriteOK(w, msg)
}

// Delete удаляет своё сообщение (для себя).
// @Summary Удалить сообщение
// @Tags messages
// @Produce json
// @Security BearerAuth
// @Param id path string true "id сообщения"
// @Success 200 {object} response.APIResponse
// @Router /messages/{id} [delete]
func (h *MessageHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	id := chi.URLParam(r, "id")
	if err := h.svc.Delete(r.Context(), id, userID); err != nil {
		writeMsgError(w, err)
		return
	}
	response.WriteOK(w, map[string]string{"status": "deleted"})
}

// MarkRead отмечает сообщения прочитанными.
// @Summary Отметить прочитанным
// @Tags messages
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param chat_id path string true "id чата"
// @Param body body domain.MarkReadRequest true "id сообщений"
// @Success 200 {object} response.APIResponse
// @Router /chats/{chat_id}/read [post]
func (h *MessageHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	chatID := chi.URLParam(r, "chat_id")
	var req domain.MarkReadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	if errs := validator.ValidateStruct(&req); len(errs) > 0 {
		response.WriteError(w, http.StatusUnprocessableEntity, "validation_error", firstErr(errs))
		return
	}
	if err := h.svc.MarkRead(r.Context(), chatID, userID, req.MessageIDs); err != nil {
		writeMsgError(w, err)
		return
	}
	response.WriteOK(w, map[string]string{"status": "read"})
}

// Unread возвращает число непрочитанных сообщений в чате.
// @Summary Непрочитанные (счётчик)
// @Tags messages
// @Produce json
// @Security BearerAuth
// @Param chat_id path string true "id чата"
// @Success 200 {object} response.APIResponse
// @Router /chats/{chat_id}/unread [get]
func (h *MessageHandler) Unread(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	chatID := chi.URLParam(r, "chat_id")
	n, err := h.svc.UnreadCount(r.Context(), chatID, userID)
	if err != nil {
		writeMsgError(w, err)
		return
	}
	response.WriteOK(w, map[string]int64{"unread": n})
}

// --- Реакции ---

// React добавляет реакцию (emoji) на сообщение.
// @Summary Реакция на сообщение
// @Tags messages
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "id сообщения"
// @Param body body domain.ReactRequest true "emoji"
// @Success 200 {object} response.APIResponse
// @Router /messages/{id}/reactions [post]
func (h *MessageHandler) React(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	id := chi.URLParam(r, "id")
	var req domain.ReactRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	if errs := validator.ValidateStruct(&req); len(errs) > 0 {
		response.WriteError(w, http.StatusUnprocessableEntity, "validation_error", firstErr(errs))
		return
	}
	if err := h.svc.React(r.Context(), id, userID, req.Emoji); err != nil {
		writeMsgError(w, err)
		return
	}
	response.WriteOK(w, map[string]string{"status": "reacted"})
}

// Unreact удаляет реакцию пользователя на сообщение.
// @Summary Убрать реакцию
// @Tags messages
// @Produce json
// @Security BearerAuth
// @Param id path string true "id сообщения"
// @Param emoji query string true "emoji"
// @Success 200 {object} response.APIResponse
// @Router /messages/{id}/reactions [delete]
func (h *MessageHandler) Unreact(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	id := chi.URLParam(r, "id")
	emoji := r.URL.Query().Get("emoji")
	if emoji == "" {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "emoji query param is required")
		return
	}
	if err := h.svc.RemoveReaction(r.Context(), id, userID, emoji); err != nil {
		writeMsgError(w, err)
		return
	}
	response.WriteOK(w, map[string]string{"status": "removed"})
}

// ListReactions возвращает все реакции на сообщение.
// @Summary Список реакций
// @Tags messages
// @Produce json
// @Security BearerAuth
// @Param id path string true "id сообщения"
// @Success 200 {object} response.APIResponse
// @Router /messages/{id}/reactions [get]
func (h *MessageHandler) ListReactions(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	reactions, err := h.svc.ListReactions(r.Context(), id)
	if err != nil {
		writeMsgError(w, err)
		return
	}
	response.WriteOK(w, reactions)
}

// --- Закрепление ---

// Pin закрепляет сообщение в чате.
// @Summary Закрепить сообщение
// @Tags messages
// @Produce json
// @Security BearerAuth
// @Param chat_id path string true "id чата"
// @Param message_id path string true "id сообщения"
// @Success 200 {object} response.APIResponse
// @Router /chats/{chat_id}/pin/{message_id} [post]
func (h *MessageHandler) Pin(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	chatID := chi.URLParam(r, "chat_id")
	messageID := chi.URLParam(r, "message_id")
	if err := h.svc.Pin(r.Context(), chatID, messageID, userID); err != nil {
		writeMsgError(w, err)
		return
	}
	response.WriteOK(w, map[string]string{"status": "pinned"})
}

// Unpin открепляет сообщение в чате.
// @Summary Открепить сообщение
// @Tags messages
// @Produce json
// @Security BearerAuth
// @Param chat_id path string true "id чата"
// @Param message_id path string true "id сообщения"
// @Success 200 {object} response.APIResponse
// @Router /chats/{chat_id}/pin/{message_id} [delete]
func (h *MessageHandler) Unpin(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	chatID := chi.URLParam(r, "chat_id")
	messageID := chi.URLParam(r, "message_id")
	if err := h.svc.Unpin(r.Context(), chatID, messageID, userID); err != nil {
		writeMsgError(w, err)
		return
	}
	response.WriteOK(w, map[string]string{"status": "unpinned"})
}

// --- Пересылка ---

// Forward пересылает сообщение в другой чат.
// @Summary Переслать сообщение
// @Tags messages
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "id сообщения"
// @Param body body domain.ForwardRequest true "id целевого чата"
// @Success 201 {object} response.APIResponse
// @Router /messages/{id}/forward [post]
func (h *MessageHandler) Forward(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	id := chi.URLParam(r, "id")
	var req domain.ForwardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	if errs := validator.ValidateStruct(&req); len(errs) > 0 {
		response.WriteError(w, http.StatusUnprocessableEntity, "validation_error", firstErr(errs))
		return
	}
	msg, err := h.svc.Forward(r.Context(), id, userID, req.ChatID)
	if err != nil {
		writeMsgError(w, err)
		return
	}
	response.WriteCreated(w, msg)
}

// --- Поиск ---

// SearchInChat ищет сообщения внутри чата.
// @Summary Поиск в чате
// @Tags messages
// @Produce json
// @Security BearerAuth
// @Param chat_id path string true "id чата"
// @Param q query string true "строка поиска"
// @Success 200 {object} response.APIResponse
// @Router /chats/{chat_id}/messages/search [get]
func (h *MessageHandler) SearchInChat(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	chatID := chi.URLParam(r, "chat_id")
	q := r.URL.Query().Get("q")
	if q == "" {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "query parameter 'q' is required")
		return
	}
	msgs, err := h.svc.SearchInChat(r.Context(), chatID, userID, q, 50)
	if err != nil {
		writeMsgError(w, err)
		return
	}
	response.WriteOK(w, msgs)
}

// SearchGlobal ищет сообщения во всех чатах пользователя.
// @Summary Глобальный поиск сообщений
// @Tags messages
// @Produce json
// @Security BearerAuth
// @Param q query string true "строка поиска"
// @Success 200 {object} response.APIResponse
// @Router /messages/search [get]
func (h *MessageHandler) SearchGlobal(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	q := r.URL.Query().Get("q")
	if q == "" {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "query parameter 'q' is required")
		return
	}
	msgs, err := h.svc.SearchGlobal(r.Context(), userID, q, 50)
	if err != nil {
		writeMsgError(w, err)
		return
	}
	response.WriteOK(w, msgs)
}

// --- Индикатор печати ---

// Typing отправляет индикатор «печатает…» участникам чата.
// @Summary Индикатор печати
// @Tags messages
// @Produce json
// @Security BearerAuth
// @Param chat_id path string true "id чата"
// @Success 200 {object} response.APIResponse
// @Router /chats/{chat_id}/typing [post]
func (h *MessageHandler) Typing(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	chatID := chi.URLParam(r, "chat_id")
	if err := h.svc.Typing(r.Context(), chatID, userID); err != nil {
		writeMsgError(w, err)
		return
	}
	response.WriteOK(w, map[string]string{"status": "typing"})
}

// DeleteForAll удаляет сообщение у всех участников (admin/owner чата).
// @Summary Удалить для всех (админ)
// @Tags messages
// @Produce json
// @Security BearerAuth
// @Param chat_id path string true "id чата"
// @Param id path string true "id сообщения"
// @Success 200 {object} response.APIResponse
// @Router /chats/{chat_id}/messages/{id}/admin [delete]
func (h *MessageHandler) DeleteForAll(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	chatID := chi.URLParam(r, "chat_id")
	id := chi.URLParam(r, "id")
	if err := h.svc.DeleteForAll(r.Context(), chatID, id, userID); err != nil {
		writeMsgError(w, err)
		return
	}
	response.WriteOK(w, map[string]string{"status": "deleted_for_all"})
}

func writeMsgError(w http.ResponseWriter, err error) {
	if err == service.ErrForbidden {
		response.WriteError(w, http.StatusForbidden, "forbidden", err.Error())
		return
	}
	if err == msgrepo.ErrMessageNotFound {
		response.WriteError(w, http.StatusNotFound, "not_found", "message not found")
		return
	}
	response.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
}

func firstErr(errs map[string]string) string {
	for _, v := range errs {
		return v
	}
	return "validation failed"
}
