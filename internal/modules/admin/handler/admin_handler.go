package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	authdomain "github.com/jeogram/messenger/internal/modules/auth/domain"
	calldomain "github.com/jeogram/messenger/internal/modules/calls/domain"
	chatdomain "github.com/jeogram/messenger/internal/modules/chat/domain"
	msgdomain "github.com/jeogram/messenger/internal/modules/message/domain"
	notifdomain "github.com/jeogram/messenger/internal/modules/notification/domain"
	"github.com/jeogram/messenger/internal/pkg/auth"
	"github.com/jeogram/messenger/internal/pkg/middleware"
	"github.com/jeogram/messenger/internal/pkg/realtime"
	"github.com/jeogram/messenger/internal/pkg/response"
	"github.com/jeogram/messenger/internal/pkg/ws"
	"gorm.io/gorm"
)

// AdminHandler exposes administrative endpoints available only to configured
// admin users. It reads data directly from the database for full visibility
// (users, their devices/IPs, chats, messages, calls and aggregate stats).
type AdminHandler struct {
	db       *gorm.DB
	jwt      *auth.JWT
	adminIDs []string
	hub      realtime.Broadcaster
}

func NewAdminHandler(db *gorm.DB, jwt *auth.JWT, adminIDs []string, hub realtime.Broadcaster) *AdminHandler {
	return &AdminHandler{db: db, jwt: jwt, adminIDs: adminIDs, hub: hub}
}

// RegisterRoutes mounts admin endpoints, each guarded by RequireAdmin.
//
//	@Summary	Admin: list users
//	@Tags		admin
//	@Produce	json
//	@Param		limit	query		int		false	"лимит"
//	@Param		offset	query		int		false	"смещение"
//	@Success	200		{object}	response.APIResponse
//	@Router		/admin/users [get]
//	@Security	BearerAuth
func (h *AdminHandler) RegisterRoutes(r chi.Router) {
	guard := middleware.RequireAdmin(h.adminIDs)
	withAdmin := func(handler http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			middleware.JWTAuth(h.jwt)(guard(handler)).ServeHTTP(w, r)
		}
	}
	r.Get("/admin/users", withAdmin(h.ListUsers))
	r.Post("/admin/users", withAdmin(h.CreateUser))
	r.Get("/admin/users/{id}", withAdmin(h.GetUser))
	r.Get("/admin/users/{id}/devices", withAdmin(h.UserDevices))
	r.Get("/admin/chats", withAdmin(h.ListChats))
	r.Get("/admin/chats/{id}/messages", withAdmin(h.ChatMessages))
	r.Get("/admin/messages/search", withAdmin(h.SearchMessages))
	r.Get("/admin/devices", withAdmin(h.ListDevices))
	r.Get("/admin/stats", withAdmin(h.Stats))
	r.Get("/admin/users/search", withAdmin(h.SearchUsers))
	r.Post("/admin/users/{id}/ban", withAdmin(h.BanUser))
	r.Post("/admin/users/{id}/unban", withAdmin(h.UnbanUser))
	r.Post("/admin/users/{id}/role", withAdmin(h.SetRole))
	r.Delete("/admin/users/{id}", withAdmin(h.DeleteUser))
	r.Post("/admin/broadcast", withAdmin(h.Broadcast))
}

func page(r *http.Request) (limit, offset int) {
	limit, _ = strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ = strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return
}

// ListUsers возвращает список пользователей с последним IP/User-Agent.
func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	limit, offset := page(r)
	var users []authdomain.User
	if err := h.db.WithContext(r.Context()).Order("created_at DESC").Limit(limit).Offset(offset).Find(&users).Error; err != nil {
		response.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	response.WriteOK(w, users)
}

// GetUser возвращает полную информацию о пользователе.
//
//	@Summary	Admin: получить пользователя по id
//	@Tags		admin
//	@Produce	json
//	@Param		id		path		string	true	"id пользователя"
//	@Success	200		{object}	response.APIResponse
//	@Router		/admin/users/{id} [get]
//	@Security	BearerAuth
func (h *AdminHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var u authdomain.User
	if err := h.db.WithContext(r.Context()).Where("id = ?", id).First(&u).Error; err != nil {
		response.WriteError(w, http.StatusNotFound, "not_found", "user not found")
		return
	}
	ctx := r.Context()
	count := func(model interface{}, where ...interface{}) int64 {
		var n int64
		q := h.db.WithContext(ctx).Model(model)
		if len(where) > 0 {
			q = q.Where(where[0], where[1:]...)
		}
		q.Count(&n)
		return n
	}
	detail := map[string]interface{}{
		"user":     u,
		"chats":    count(&chatdomain.Chat{}, "id IN (SELECT chat_id FROM chat_participants WHERE user_id = ?)", id),
		"messages": count(&msgdomain.Message{}, "sender_id = ? AND deleted_at IS NULL", id),
		"devices":  count(&notifdomain.DeviceToken{}, "user_id = ?", id),
		"calls":    count(&calldomain.Call{}, "initiator = ?", id),
	}
	response.WriteOK(w, detail)
}

// UserDevices возвращает устройства пользователя (модель, ОС, IP, локаль и т.п.).
//
//	@Summary	Admin: устройства пользователя
//	@Tags		admin
//	@Produce	json
//	@Param		id		path		string	true	"id пользователя"
//	@Success	200		{object}	response.APIResponse
//	@Router		/admin/users/{id}/devices [get]
//	@Security	BearerAuth
func (h *AdminHandler) UserDevices(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var devices []notifdomain.DeviceToken
	if err := h.db.WithContext(r.Context()).Where("user_id = ?", id).Find(&devices).Error; err != nil {
		response.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	response.WriteOK(w, devices)
}

// ListChats возвращает все чаты системы.
//
//	@Summary	Admin: список всех чатов
//	@Tags		admin
//	@Produce	json
//	@Param		limit	query		int		false	"лимит"
//	@Param		offset	query		int		false	"смещение"
//	@Success	200		{object}	response.APIResponse
//	@Router		/admin/chats [get]
//	@Security	BearerAuth
func (h *AdminHandler) ListChats(w http.ResponseWriter, r *http.Request) {
	limit, offset := page(r)
	var chats []chatdomain.Chat
	if err := h.db.WithContext(r.Context()).Order("created_at DESC").Limit(limit).Offset(offset).Find(&chats).Error; err != nil {
		response.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	response.WriteOK(w, chats)
}

// ChatMessages возвращает сообщения конкретного чата.
//
//	@Summary	Admin: сообщения чата
//	@Tags		admin
//	@Produce	json
//	@Param		id		path		string	true	"id чата"
//	@Param		limit	query		int		false	"лимит"
//	@Param		offset	query		int		false	"смещение"
//	@Success	200		{object}	response.APIResponse
//	@Router		/admin/chats/{id}/messages [get]
//	@Security	BearerAuth
func (h *AdminHandler) ChatMessages(w http.ResponseWriter, r *http.Request) {
	chatID := chi.URLParam(r, "id")
	limit, offset := page(r)
	var msgs []msgdomain.Message
	if err := h.db.WithContext(r.Context()).
		Where("chat_id = ? AND deleted_at IS NULL", chatID).
		Order("created_at DESC").Limit(limit).Offset(offset).
		Find(&msgs).Error; err != nil {
		response.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	response.WriteOK(w, msgs)
}

// SearchMessages ищет сообщения по тексту по всей системе.
//
//	@Summary	Admin: поиск сообщений по тексту
//	@Tags		admin
//	@Produce	json
//	@Param		q		query		string	true	"поисковый запрос"
//	@Param		limit	query		int		false	"лимит"
//	@Success	200		{object}	response.APIResponse
//	@Router		/admin/messages/search [get]
//	@Security	BearerAuth
func (h *AdminHandler) SearchMessages(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "query parameter 'q' is required")
		return
	}
	limit, _ := page(r)
	var msgs []msgdomain.Message
	if err := h.db.WithContext(r.Context()).
		Where("text LIKE ? AND deleted_at IS NULL", "%"+q+"%").
		Order("created_at DESC").Limit(limit).
		Find(&msgs).Error; err != nil {
		response.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	response.WriteOK(w, msgs)
}

// ListDevices возвращает все зарегистрированные устройства (IP, модель, ОС).
//
//	@Summary	Admin: все устройства (IP, модель, ОС)
//	@Tags		admin
//	@Produce	json
//	@Param		limit	query		int		false	"лимит"
//	@Param		offset	query		int		false	"смещение"
//	@Success	200		{object}	response.APIResponse
//	@Router		/admin/devices [get]
//	@Security	BearerAuth
func (h *AdminHandler) ListDevices(w http.ResponseWriter, r *http.Request) {
	limit, offset := page(r)
	var devices []notifdomain.DeviceToken
	if err := h.db.WithContext(r.Context()).Order("updated_at DESC").Limit(limit).Offset(offset).Find(&devices).Error; err != nil {
		response.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	response.WriteOK(w, devices)
}

// Stats возвращает агрегированную статистику системы.
//
//	@Summary	Admin: aggregate stats
//	@Tags		admin
//	@Produce	json
//	@Success	200	{object}	response.APIResponse
//	@Router		/admin/stats [get]
//	@Security	BearerAuth
func (h *AdminHandler) Stats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	count := func(model interface{}) int64 {
		var n int64
		h.db.WithContext(ctx).Model(model).Count(&n)
		return n
	}
	stats := map[string]int64{
		"users":    count(&authdomain.User{}),
		"chats":    count(&chatdomain.Chat{}),
		"messages": count(&msgdomain.Message{}),
		"calls":    count(&calldomain.Call{}),
		"devices":  count(&notifdomain.DeviceToken{}),
	}
	response.WriteOK(w, stats)
}

// SearchUsers ищет пользователей по username/display_name/email.
//
//	@Summary	Admin: поиск пользователей
//	@Tags		admin
//	@Produce	json
//	@Param		q		query		string	true	"запрос"
//	@Param		limit	query		int		false	"лимит"
//	@Success	200		{object}	response.APIResponse
//	@Router		/admin/users/search [get]
//	@Security	BearerAuth
func (h *AdminHandler) SearchUsers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "query parameter 'q' is required")
		return
	}
	limit, _ := page(r)
	var users []authdomain.User
	like := "%" + q + "%"
	if err := h.db.WithContext(r.Context()).
		Where("LOWER(username) LIKE LOWER(?) OR LOWER(display_name) LIKE LOWER(?) OR LOWER(email) LIKE LOWER(?)", like, like, like).
		Order("created_at DESC").Limit(limit).Find(&users).Error; err != nil {
		response.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	response.WriteOK(w, users)
}

// CreateUser создаёт аккаунт (в том числе админский) от лица администратора.
//
//	@Summary	Admin: создать пользователя (включая админа)
//	@Tags		admin
//	@Accept		json
//	@Produce	json
//	@Param		body	body	createUserRequest	true	"данные аккаунта"
//	@Success	201		{object}	response.APIResponse
//	@Router		/admin/users [post]
//	@Security	BearerAuth
func (h *AdminHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" || req.Username == "" || req.Password == "" {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "email, username and password required")
		return
	}
	role := req.Role
	if role == "" {
		role = authdomain.RoleUser
	}
	if role != authdomain.RoleUser && role != authdomain.RoleAdmin {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "invalid role")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	u := authdomain.User{
		Email:        req.Email,
		Username:     req.Username,
		Phone:        req.Phone,
		PasswordHash: hash,
		DisplayName:  req.Username,
		Status:       authdomain.StatusActive,
		Role:         role,
	}
	// Проверка уникальности перед вставкой.
	var dup int64
	h.db.WithContext(r.Context()).Model(&authdomain.User{}).
		Where("email = ? OR username = ?", req.Email, req.Username).Count(&dup)
	if dup > 0 {
		response.WriteError(w, http.StatusConflict, "conflict", "email or username already in use")
		return
	}
	if err := h.db.WithContext(r.Context()).Create(&u).Error; err != nil {
		response.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	response.WriteCreated(w, u)
}

type createUserRequest struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
	Phone    string `json:"phone,omitempty"`
	Role     string `json:"role,omitempty"` // user | admin
}

func (h *AdminHandler) loadUser(w http.ResponseWriter, r *http.Request) (*authdomain.User, bool) {
	id := chi.URLParam(r, "id")
	var u authdomain.User
	if err := h.db.WithContext(r.Context()).Where("id = ?", id).First(&u).Error; err != nil {
		response.WriteError(w, http.StatusNotFound, "not_found", "user not found")
		return nil, false
	}
	return &u, true
}

// BanUser блокирует пользователя.
//
//	@Summary	Admin: заблокировать пользователя
//	@Tags		admin
//	@Produce	json
//	@Param		id		path		string	true	"id пользователя"
//	@Success	200		{object}	response.APIResponse
//	@Router		/admin/users/{id}/ban [post]
//	@Security	BearerAuth
func (h *AdminHandler) BanUser(w http.ResponseWriter, r *http.Request) {
	u, ok := h.loadUser(w, r)
	if !ok {
		return
	}
	u.Status = authdomain.StatusBanned
	if err := h.db.WithContext(r.Context()).Save(u).Error; err != nil {
		response.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	response.WriteOK(w, map[string]string{"status": "banned", "user_id": u.ID})
}

// UnbanUser разблокирует пользователя.
//
//	@Summary	Admin: разблокировать пользователя
//	@Tags		admin
//	@Produce	json
//	@Param		id		path		string	true	"id пользователя"
//	@Success	200		{object}	response.APIResponse
//	@Router		/admin/users/{id}/unban [post]
//	@Security	BearerAuth
func (h *AdminHandler) UnbanUser(w http.ResponseWriter, r *http.Request) {
	u, ok := h.loadUser(w, r)
	if !ok {
		return
	}
	u.Status = authdomain.StatusActive
	if err := h.db.WithContext(r.Context()).Save(u).Error; err != nil {
		response.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	response.WriteOK(w, map[string]string{"status": "active", "user_id": u.ID})
}

// SetRole меняет роль пользователя (user/admin).
//
//	@Summary	Admin: назначить роль
//	@Tags		admin
//	@Accept		json
//	@Produce	json
//	@Param		id		path		string	true	"id пользователя"
//	@Param		body	body		setRoleRequest	true	"роль"
//	@Success	200		{object}	response.APIResponse
//	@Router		/admin/users/{id}/role [post]
//	@Security	BearerAuth
func (h *AdminHandler) SetRole(w http.ResponseWriter, r *http.Request) {
	u, ok := h.loadUser(w, r)
	if !ok {
		return
	}
	var req setRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Role == "" {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "role required")
		return
	}
	if req.Role != authdomain.RoleUser && req.Role != authdomain.RoleAdmin {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "invalid role")
		return
	}
	u.Role = req.Role
	if err := h.db.WithContext(r.Context()).Save(u).Error; err != nil {
		response.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	response.WriteOK(w, map[string]string{"role": u.Role, "user_id": u.ID})
}

type setRoleRequest struct {
	Role string `json:"role"`
}

// DeleteUser удаляет аккаунт пользователя.
//
//	@Summary	Admin: удалить пользователя
//	@Tags		admin
//	@Produce	json
//	@Param		id		path		string	true	"id пользователя"
//	@Success	200		{object}	response.APIResponse
//	@Router		/admin/users/{id} [delete]
//	@Security	BearerAuth
func (h *AdminHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.db.WithContext(r.Context()).Where("id = ?", id).Delete(&authdomain.User{}).Error; err != nil {
		response.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	response.WriteOK(w, map[string]string{"status": "deleted", "user_id": id})
}

// Broadcast отправляет системное уведомление всем подключённым пользователям.
//
//	@Summary	Admin: рассылка уведомления всем онлайн-пользователям
//	@Tags		admin
//	@Accept		json
//	@Produce	json
//	@Param		body	body		broadcastRequest	true	"заголовок и текст"
//	@Success	200		{object}	response.APIResponse
//	@Router		/admin/broadcast [post]
//	@Security	BearerAuth
func (h *AdminHandler) Broadcast(w http.ResponseWriter, r *http.Request) {
	var req broadcastRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Title == "" {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "title required")
		return
	}
	if h.hub == nil {
		response.WriteOK(w, map[string]string{"status": "no_realtime_hub"})
		return
	}
	ids := h.hub.OnlineUserIDs()
	payload := map[string]interface{}{"title": req.Title, "body": req.Body}
	h.hub.SendToUsers(ids, ws.Outbound{Type: "admin.broadcast", Payload: payload})
	response.WriteOK(w, map[string]interface{}{"status": "broadcast", "recipients": len(ids)})
}

type broadcastRequest struct {
	Title string `json:"title"`
	Body  string `json:"body,omitempty"`
}
