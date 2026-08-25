package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jeogram/messenger/internal/modules/contact/domain"
	"github.com/jeogram/messenger/internal/modules/contact/service"
	"github.com/jeogram/messenger/internal/pkg/auth"
	"github.com/jeogram/messenger/internal/pkg/middleware"
	"github.com/jeogram/messenger/internal/pkg/response"
	"github.com/jeogram/messenger/internal/pkg/validator"
)

// ContactHandler exposes contact (friends) endpoints.
type ContactHandler struct {
	svc *service.ContactService
	jwt *auth.JWT
}

func NewContactHandler(svc *service.ContactService, jwt *auth.JWT) *ContactHandler {
	return &ContactHandler{svc: svc, jwt: jwt}
}

// RegisterRoutes mounts contact endpoints.
//
//	@Summary	Add a user to contacts
//	@Tags		contacts
//	@Accept		json
//	@Produce	json
//	@Param		body	body	domain.AddContactRequest	true	"id пользователя"
//	@Success	201	{object}	response.APIResponse
//	@Router		/contacts [post]
//	@Security	BearerAuth
func (h *ContactHandler) RegisterRoutes(r chi.Router) {
	r.With(middleware.JWTAuth(h.jwt)).Post("/contacts", h.Add)
	r.With(middleware.JWTAuth(h.jwt)).Get("/contacts", h.List)
	r.With(middleware.JWTAuth(h.jwt)).Get("/contacts/requests", h.ListRequests)
	r.With(middleware.JWTAuth(h.jwt)).Post("/contacts/{user_id}/accept", h.Accept)
	r.With(middleware.JWTAuth(h.jwt)).Delete("/contacts/{user_id}", h.Remove)
	r.With(middleware.JWTAuth(h.jwt)).Get("/contacts/{user_id}", h.Get)
	r.With(middleware.JWTAuth(h.jwt)).Post("/contacts/sync", h.Sync)
	r.With(middleware.JWTAuth(h.jwt)).Post("/contacts/{user_id}/block", h.BlockContact)
}

// Add отправляет запрос в контакты.
// @Summary Отправить запрос в контакты
// @Tags contacts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body domain.AddContactRequest true "id пользователя"
// @Success 201 {object} response.APIResponse
// @Router /contacts [post]
func (h *ContactHandler) Add(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	var req domain.AddContactRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	if errs := validator.ValidateStruct(&req); len(errs) > 0 {
		response.WriteError(w, http.StatusUnprocessableEntity, "validation_error", firstContactErr(errs))
		return
	}
	c, err := h.svc.Add(r.Context(), userID, req.UserID)
	if err != nil {
		writeContactError(w, err)
		return
	}
	response.WriteCreated(w, c)
}

// Get возвращает запись контакта с конкретным пользователем.
//
//	@Summary	Получить контакт по id пользователя
//	@Tags		contacts
//	@Produce	json
//	@Param		user_id	path		string	true	"id пользователя"
//	@Success	200		{object}	response.APIResponse
//	@Router		/contacts/{user_id} [get]
//	@Security	BearerAuth
func (h *ContactHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	target := chi.URLParam(r, "user_id")
	c, err := h.svc.Get(r.Context(), userID, target)
	if err != nil {
		writeContactError(w, err)
		return
	}
	response.WriteOK(w, c)
}

// List возвращает список контактов (подтверждённых).
// @Summary Список контактов
// @Tags contacts
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /contacts [get]
func (h *ContactHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	users, err := h.svc.List(r.Context(), userID)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	response.WriteOK(w, users)
}

// ListRequests возвращает входящие запросы в контакты.
// @Summary Входящие запросы
// @Tags contacts
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /contacts/requests [get]
func (h *ContactHandler) ListRequests(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	users, err := h.svc.ListRequests(r.Context(), userID)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	response.WriteOK(w, users)
}

// Accept подтверждает входящий запрос в контакты.
// @Summary Принять запрос в контакты
// @Tags contacts
// @Produce json
// @Security BearerAuth
// @Param user_id path string true "id отправителя запроса"
// @Success 200 {object} response.APIResponse
// @Router /contacts/{user_id}/accept [post]
func (h *ContactHandler) Accept(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	requester := chi.URLParam(r, "user_id")
	if err := h.svc.Accept(r.Context(), userID, requester); err != nil {
		writeContactError(w, err)
		return
	}
	response.WriteOK(w, map[string]string{"status": "accepted"})
}

// Remove удаляет пользователя из контактов.
// @Summary Удалить из контактов
// @Tags contacts
// @Produce json
// @Security BearerAuth
// @Param user_id path string true "id пользователя"
// @Success 200 {object} response.APIResponse
// @Router /contacts/{user_id} [delete]
func (h *ContactHandler) Remove(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	target := chi.URLParam(r, "user_id")
	if err := h.svc.Remove(r.Context(), userID, target); err != nil {
		writeContactError(w, err)
		return
	}
	response.WriteOK(w, map[string]string{"status": "removed"})
}

type syncRequest struct {
	UserIDs []string `json:"user_ids" validate:"required"`
}

// Sync массово добавляет контакты из телефонной книги по списку user_id.
// @Summary Синхронизация контактов (массовое добавление)
// @Tags contacts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body syncRequest true "user_ids"
// @Success 200 {object} response.APIResponse
// @Router /contacts/sync [post]
func (h *ContactHandler) Sync(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	var req syncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.UserIDs) == 0 {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "user_ids required")
		return
	}
	added, err := h.svc.Sync(r.Context(), userID, req.UserIDs)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	response.WriteOK(w, map[string]int{"added": added})
}

// BlockContact блокирует пользователя (привязка к чёрному списку аккаунта).
// @Summary Заблокировать пользователя
// @Tags contacts
// @Produce json
// @Security BearerAuth
// @Param user_id path string true "id пользователя"
// @Success 200 {object} response.APIResponse
// @Router /contacts/{user_id}/block [post]
func (h *ContactHandler) BlockContact(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	target := chi.URLParam(r, "user_id")
	if err := h.svc.Block(r.Context(), userID, target); err != nil {
		writeContactError(w, err)
		return
	}
	response.WriteOK(w, map[string]string{"status": "blocked"})
}

func writeContactError(w http.ResponseWriter, err error) {
	response.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
}

func firstContactErr(errs map[string]string) string {
	for _, v := range errs {
		return v
	}
	return "validation failed"
}
