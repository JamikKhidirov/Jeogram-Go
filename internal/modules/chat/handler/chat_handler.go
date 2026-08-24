package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jeogram/messenger/internal/modules/chat/domain"
	chatrepo "github.com/jeogram/messenger/internal/modules/chat/repository"
	"github.com/jeogram/messenger/internal/modules/chat/service"
	"github.com/jeogram/messenger/internal/pkg/auth"
	"github.com/jeogram/messenger/internal/pkg/middleware"
	"github.com/jeogram/messenger/internal/pkg/response"
	"github.com/jeogram/messenger/internal/pkg/validator"
)

// ChatHandler exposes chat endpoints.
type ChatHandler struct {
	svc *service.ChatService
	jwt *auth.JWT
}

func NewChatHandler(svc *service.ChatService, jwt *auth.JWT) *ChatHandler {
	return &ChatHandler{svc: svc, jwt: jwt}
}

// RegisterRoutes mounts chat endpoints.
//
//	@Summary	List my chats
//	@Tags		chat
//	@Produce	json
//	@Success	200	{object}	response.APIResponse
//	@Router		/chats [get]
//	@Security	BearerAuth
func (h *ChatHandler) RegisterRoutes(r chi.Router) {
	r.With(middleware.JWTAuth(h.jwt)).Get("/chats", h.List)
	r.With(middleware.JWTAuth(h.jwt)).Post("/chats/private", h.CreatePrivate)
	r.With(middleware.JWTAuth(h.jwt)).Post("/chats/group", h.CreateGroup)
	r.With(middleware.JWTAuth(h.jwt)).Get("/chats/{chat_id}", h.Get)
	r.With(middleware.JWTAuth(h.jwt)).Put("/chats/{chat_id}", h.UpdateChat)
	r.With(middleware.JWTAuth(h.jwt)).Post("/chats/{chat_id}/participants", h.AddParticipant)
	r.With(middleware.JWTAuth(h.jwt)).Post("/chats/{chat_id}/participants/{user_id}/promote", h.Promote)
	r.With(middleware.JWTAuth(h.jwt)).Post("/chats/{chat_id}/participants/{user_id}/demote", h.Demote)
	r.With(middleware.JWTAuth(h.jwt)).Delete("/chats/{chat_id}/participants/{user_id}", h.RemoveParticipant)
	r.With(middleware.JWTAuth(h.jwt)).Get("/chats/{chat_id}/participants", h.Participants)
	r.With(middleware.JWTAuth(h.jwt)).Get("/chats/search", h.Search)
	r.With(middleware.JWTAuth(h.jwt)).Post("/chats/{chat_id}/leave", h.Leave)
	r.With(middleware.JWTAuth(h.jwt)).Post("/chats/{chat_id}/e2ee/enable", h.EnableE2EE)
	r.With(middleware.JWTAuth(h.jwt)).Post("/chats/{chat_id}/mute", h.Mute)
	r.With(middleware.JWTAuth(h.jwt)).Delete("/chats/{chat_id}/mute", h.Unmute)
}

// Search ищет чаты пользователя по названию.
// @Summary Поиск чатов
// @Tags chat
// @Produce json
// @Security BearerAuth
// @Param q query string true "строка поиска"
// @Success 200 {object} response.APIResponse
// @Router /chats/search [get]
func (h *ChatHandler) Search(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	q := r.URL.Query().Get("q")
	if q == "" {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "query parameter 'q' is required")
		return
	}
	chats, err := h.svc.SearchChats(r.Context(), userID, q, 20)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	response.WriteOK(w, chats)
}

// Leave покидает чат (удаляет пользователя из участников).
// @Summary Покинуть чат
// @Tags chat
// @Produce json
// @Security BearerAuth
// @Param chat_id path string true "id чата"
// @Success 200 {object} response.APIResponse
// @Router /chats/{chat_id}/leave [post]
func (h *ChatHandler) Leave(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	chatID := chi.URLParam(r, "chat_id")
	if err := h.svc.LeaveChat(r.Context(), userID, chatID); err != nil {
		writeChatError(w, err)
		return
	}
	response.WriteOK(w, map[string]string{"status": "left"})
}

// EnableE2EE включает сквозное шифрование для чата (сервер хранит только ciphertext).
// @Summary Включить E2EE для чата
// @Tags chat
// @Produce json
// @Security BearerAuth
// @Param chat_id path string true "id чата"
// @Success 200 {object} response.APIResponse
// @Router /chats/{chat_id}/e2ee/enable [post]
func (h *ChatHandler) EnableE2EE(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	chatID := chi.URLParam(r, "chat_id")
	chat, err := h.svc.EnableE2EE(r.Context(), chatID, userID)
	if err != nil {
		writeChatError(w, err)
		return
	}
	response.WriteOK(w, chat)
}

// List возвращает список чатов пользователя.
// @Summary Список моих чатов
// @Tags chat
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /chats [get]
func (h *ChatHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	chats, err := h.svc.ListChats(r.Context(), userID)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	response.WriteOK(w, chats)
}

// Get возвращает информацию о конкретном чате.
// @Summary Информация о чате
// @Tags chat
// @Produce json
// @Security BearerAuth
// @Param chat_id path string true "id чата"
// @Success 200 {object} response.APIResponse
// @Router /chats/{chat_id} [get]
func (h *ChatHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	chatID := chi.URLParam(r, "chat_id")
	chat, err := h.svc.GetChat(r.Context(), chatID, userID)
	if err != nil {
		writeChatError(w, err)
		return
	}
	response.WriteOK(w, chat)
}

// CreatePrivate создаёт приватный чат с пользователем.
// @Summary Создать приватный чат
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body domain.CreatePrivateChatRequest true "id собеседника"
// @Success 201 {object} response.APIResponse
// @Router /chats/private [post]
func (h *ChatHandler) CreatePrivate(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	var req domain.CreatePrivateChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	if errs := validator.ValidateStruct(&req); len(errs) > 0 {
		response.WriteError(w, http.StatusUnprocessableEntity, "validation_error", firstErr(errs))
		return
	}
	chat, err := h.svc.CreatePrivateChat(r.Context(), userID, req.UserID)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	response.WriteCreated(w, chat)
}

// CreateGroup создаёт групповой чат.
// @Summary Создать групповой чат
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body domain.CreateGroupChatRequest true "название и участники"
// @Success 201 {object} response.APIResponse
// @Router /chats/group [post]
func (h *ChatHandler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	var req domain.CreateGroupChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	if errs := validator.ValidateStruct(&req); len(errs) > 0 {
		response.WriteError(w, http.StatusUnprocessableEntity, "validation_error", firstErr(errs))
		return
	}
	chat, err := h.svc.CreateGroupChat(r.Context(), userID, req)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	response.WriteCreated(w, chat)
}

// AddParticipant добавляет участника в групповой чат (только owner/admin).
// @Summary Добавить участника
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param chat_id path string true "id чата"
// @Param body body object true "поле user_id"
// @Success 201 {object} response.APIResponse
// @Router /chats/{chat_id}/participants [post]
func (h *ChatHandler) AddParticipant(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	chatID := chi.URLParam(r, "chat_id")
	var body struct {
		UserID string `json:"user_id" validate:"required"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	if errs := validator.ValidateStruct(&body); len(errs) > 0 {
		response.WriteError(w, http.StatusUnprocessableEntity, "validation_error", firstErr(errs))
		return
	}
	chat, err := h.svc.AddParticipant(r.Context(), chatID, body.UserID, userID)
	if err != nil {
		writeChatError(w, err)
		return
	}
	response.WriteCreated(w, chat)
}

// Participants возвращает список участников чата.
// @Summary Участники чата
// @Tags chat
// @Produce json
// @Security BearerAuth
// @Param chat_id path string true "id чата"
// @Success 200 {object} response.APIResponse
// @Router /chats/{chat_id}/participants [get]
func (h *ChatHandler) Participants(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	chatID := chi.URLParam(r, "chat_id")
	ids, err := h.svc.Participants(r.Context(), chatID, userID)
	if err != nil {
		writeChatError(w, err)
		return
	}
	response.WriteOK(w, ids)
}

// UpdateChat обновляет название/аватар группового чата (только owner/admin).
// @Summary Обновить чат
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param chat_id path string true "id чата"
// @Param body body object false "поля title, avatar_url"
// @Success 200 {object} response.APIResponse
// @Router /chats/{chat_id} [put]
func (h *ChatHandler) UpdateChat(w http.ResponseWriter, r *http.Request) {
	requester := middleware.UserID(r)
	chatID := chi.URLParam(r, "chat_id")
	var body struct {
		Title     string `json:"title"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	chat, err := h.svc.UpdateChat(r.Context(), chatID, requester, body.Title, body.AvatarURL)
	if err != nil {
		writeChatError(w, err)
		return
	}
	response.WriteOK(w, chat)
}

// Promote назначает участника администратором (только owner).
// @Summary Назначить админа
// @Tags chat
// @Produce json
// @Security BearerAuth
// @Param chat_id path string true "id чата"
// @Param user_id path string true "id участника"
// @Success 200 {object} response.APIResponse
// @Router /chats/{chat_id}/participants/{user_id}/promote [post]
func (h *ChatHandler) Promote(w http.ResponseWriter, r *http.Request) {
	requester := middleware.UserID(r)
	chatID := chi.URLParam(r, "chat_id")
	target := chi.URLParam(r, "user_id")
	if err := h.svc.Promote(r.Context(), chatID, requester, target); err != nil {
		writeChatError(w, err)
		return
	}
	response.WriteOK(w, map[string]string{"status": "promoted"})
}

// Demote снимает права администратора (только owner).
// @Summary Снять админа
// @Tags chat
// @Produce json
// @Security BearerAuth
// @Param chat_id path string true "id чата"
// @Param user_id path string true "id участника"
// @Success 200 {object} response.APIResponse
// @Router /chats/{chat_id}/participants/{user_id}/demote [post]
func (h *ChatHandler) Demote(w http.ResponseWriter, r *http.Request) {
	requester := middleware.UserID(r)
	chatID := chi.URLParam(r, "chat_id")
	target := chi.URLParam(r, "user_id")
	if err := h.svc.Demote(r.Context(), chatID, requester, target); err != nil {
		writeChatError(w, err)
		return
	}
	response.WriteOK(w, map[string]string{"status": "demoted"})
}

// RemoveParticipant удаляет участника из чата (owner/admin или сам пользователь).
// @Summary Удалить участника
// @Tags chat
// @Produce json
// @Security BearerAuth
// @Param chat_id path string true "id чата"
// @Param user_id path string true "id участника"
// @Success 200 {object} response.APIResponse
// @Router /chats/{chat_id}/participants/{user_id} [delete]
func (h *ChatHandler) RemoveParticipant(w http.ResponseWriter, r *http.Request) {
	requester := middleware.UserID(r)
	chatID := chi.URLParam(r, "chat_id")
	target := chi.URLParam(r, "user_id")
	if err := h.svc.RemoveParticipant(r.Context(), chatID, requester, target); err != nil {
		writeChatError(w, err)
		return
	}
	response.WriteOK(w, map[string]string{"status": "removed"})
}

// Mute заглушает уведомления чата для текущего пользователя.
// @Summary Заглушить уведомления чата
// @Tags chat
// @Produce json
// @Security BearerAuth
// @Param chat_id path string true "id чата"
// @Success 200 {object} response.APIResponse
// @Router /chats/{chat_id}/mute [post]
func (h *ChatHandler) Mute(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	chatID := chi.URLParam(r, "chat_id")
	if err := h.svc.MuteChat(r.Context(), userID, chatID, true); err != nil {
		writeChatError(w, err)
		return
	}
	response.WriteOK(w, map[string]string{"status": "muted"})
}

// Unmute возобновляет уведомления чата для текущего пользователя.
// @Summary Включить уведомления чата
// @Tags chat
// @Produce json
// @Security BearerAuth
// @Param chat_id path string true "id чата"
// @Success 200 {object} response.APIResponse
// @Router /chats/{chat_id}/mute [delete]
func (h *ChatHandler) Unmute(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	chatID := chi.URLParam(r, "chat_id")
	if err := h.svc.MuteChat(r.Context(), userID, chatID, false); err != nil {
		writeChatError(w, err)
		return
	}
	response.WriteOK(w, map[string]string{"status": "unmuted"})
}

func writeChatError(w http.ResponseWriter, err error) {
	switch err {
	case chatrepo.ErrNotParticipant:
		response.WriteError(w, http.StatusForbidden, "forbidden", err.Error())
	case chatrepo.ErrChatNotFound:
		response.WriteError(w, http.StatusNotFound, "not_found", "chat not found")
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
