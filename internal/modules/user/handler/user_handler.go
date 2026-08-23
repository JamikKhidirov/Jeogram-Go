package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jeogram/messenger/internal/modules/user/domain"
	"github.com/jeogram/messenger/internal/modules/user/service"
	"github.com/jeogram/messenger/internal/pkg/auth"
	"github.com/jeogram/messenger/internal/pkg/middleware"
	"github.com/jeogram/messenger/internal/pkg/response"
	"github.com/jeogram/messenger/internal/pkg/validator"
	"github.com/jeogram/messenger/internal/pkg/ws"
)

// UserHandler exposes profile/settings/search endpoints.
type UserHandler struct {
	svc *service.UserService
	jwt *auth.JWT
	hub *ws.Hub
}

func NewUserHandler(svc *service.UserService, jwt *auth.JWT, hub *ws.Hub) *UserHandler {
	return &UserHandler{svc: svc, jwt: jwt, hub: hub}
}

// RegisterRoutes mounts user endpoints.
//
//	@Summary	Get current user's profile
//	@Tags		user
//	@Produce	json
//	@Success	200	{object}	response.APIResponse
//	@Router		/user/profile [get]
//	@Security	BearerAuth
func (h *UserHandler) RegisterRoutes(r chi.Router) {
	r.With(middleware.JWTAuth(h.jwt)).Get("/user/profile", h.GetProfile)
	r.With(middleware.JWTAuth(h.jwt)).Put("/user/profile", h.UpdateProfile)
	r.With(middleware.JWTAuth(h.jwt)).Get("/user/settings", h.GetSettings)
	r.With(middleware.JWTAuth(h.jwt)).Put("/user/settings", h.UpdateSettings)
	r.With(middleware.JWTAuth(h.jwt)).Get("/user/search", h.Search)
	r.With(middleware.JWTAuth(h.jwt)).Post("/user/block", h.Block)
	r.With(middleware.JWTAuth(h.jwt)).Delete("/user/block/{user_id}", h.Unblock)
	r.With(middleware.JWTAuth(h.jwt)).Get("/user/blocks", h.ListBlocks)
	r.With(middleware.JWTAuth(h.jwt)).Get("/user/presence", h.Presence)
	r.With(middleware.JWTAuth(h.jwt)).Delete("/user/account", h.DeleteAccount)
}

// GetProfile возвращает профиль текущего пользователя.
// @Summary Профиль пользователя
// @Tags user
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /user/profile [get]
func (h *UserHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	u, err := h.svc.GetProfile(r.Context(), userID)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}
	response.WriteOK(w, u)
}

// UpdateProfile обновляет профиль текущего пользователя.
// @Summary Обновить профиль
// @Tags user
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body domain.UpdateProfileRequest true "поля профиля"
// @Success 200 {object} response.APIResponse
// @Router /user/profile [put]
func (h *UserHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	var req domain.UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	if errs := validator.ValidateStruct(&req); len(errs) > 0 {
		response.WriteError(w, http.StatusUnprocessableEntity, "validation_error", firstErr(errs))
		return
	}
	u, err := h.svc.UpdateProfile(r.Context(), userID, req)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	response.WriteOK(w, u)
}

// GetSettings возвращает настройки пользователя.
// @Summary Настройки пользователя
// @Tags user
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /user/settings [get]
func (h *UserHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	s, err := h.svc.GetSettings(r.Context(), userID)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	response.WriteOK(w, s)
}

// UpdateSettings обновляет настройки пользователя.
// @Summary Обновить настройки
// @Tags user
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body domain.UpdateSettingsRequest true "настройки"
// @Success 200 {object} response.APIResponse
// @Router /user/settings [put]
func (h *UserHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	var req domain.UpdateSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	if errs := validator.ValidateStruct(&req); len(errs) > 0 {
		response.WriteError(w, http.StatusUnprocessableEntity, "validation_error", firstErr(errs))
		return
	}
	s, err := h.svc.UpdateSettings(r.Context(), userID, req)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	response.WriteOK(w, s)
}

// Search ищет пользователей по имени/никнейму/телефону.
// @Summary Поиск пользователей
// @Tags user
// @Produce json
// @Security BearerAuth
// @Param q query string true "строка поиска"
// @Success 200 {object} response.APIResponse
// @Router /user/search [get]
func (h *UserHandler) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "query parameter 'q' is required")
		return
	}
	limit := 20
	users, err := h.svc.Search(r.Context(), q, limit)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	response.WriteOK(w, users)
}

// Block блокирует другого пользователя.
// @Summary Заблокировать пользователя
// @Tags user
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body domain.BlockRequest true "id пользователя"
// @Success 200 {object} response.APIResponse
// @Router /user/block [post]
func (h *UserHandler) Block(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	var req domain.BlockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	if errs := validator.ValidateStruct(&req); len(errs) > 0 {
		response.WriteError(w, http.StatusUnprocessableEntity, "validation_error", firstErr(errs))
		return
	}
	if err := h.svc.Block(r.Context(), userID, req.UserID); err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	response.WriteOK(w, map[string]string{"status": "blocked"})
}

// Unblock разблокирует пользователя.
// @Summary Разблокировать пользователя
// @Tags user
// @Produce json
// @Security BearerAuth
// @Param user_id path string true "id пользователя"
// @Success 200 {object} response.APIResponse
// @Router /user/block/{user_id} [delete]
func (h *UserHandler) Unblock(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	blockedID := chi.URLParam(r, "user_id")
	if err := h.svc.Unblock(r.Context(), userID, blockedID); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	response.WriteOK(w, map[string]string{"status": "unblocked"})
}

// ListBlocks возвращает список заблокированных пользователей.
// @Summary Список заблокированных
// @Tags user
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /user/blocks [get]
func (h *UserHandler) ListBlocks(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	blocks, err := h.svc.ListBlocks(r.Context(), userID)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	response.WriteOK(w, blocks)
}

// Presence возвращает онлайн-статус пользователей.
// @Summary Онлайн-статус пользователей
// @Tags user
// @Produce json
// @Security BearerAuth
// @Param ids query string false "список id через запятую (если пусто — все онлайн)"
// @Success 200 {object} response.APIResponse
// @Router /user/presence [get]
func (h *UserHandler) Presence(w http.ResponseWriter, r *http.Request) {
	idsParam := r.URL.Query().Get("ids")
	online := map[string]bool{}
	if idsParam == "" {
		for _, id := range h.hub.OnlineUserIDs() {
			online[id] = true
		}
	} else {
		for _, id := range strings.Split(idsParam, ",") {
			id = strings.TrimSpace(id)
			if id != "" {
				online[id] = h.hub.IsOnline(id)
			}
		}
	}
	response.WriteOK(w, map[string]interface{}{"online": online})
}

// DeleteAccount удаляет аккаунт текущего пользователя и все связанные данные.
// @Summary Удалить аккаунт
// @Tags user
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /user/account [delete]
func (h *UserHandler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	if err := h.svc.DeleteAccount(r.Context(), userID); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
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
