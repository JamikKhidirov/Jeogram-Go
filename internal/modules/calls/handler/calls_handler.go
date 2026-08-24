package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/jeogram/messenger/internal/modules/calls/domain"
	"github.com/jeogram/messenger/internal/modules/calls/service"
	chatrepo "github.com/jeogram/messenger/internal/modules/chat/repository"
	"github.com/jeogram/messenger/internal/pkg/auth"
	"github.com/jeogram/messenger/internal/pkg/middleware"
	"github.com/jeogram/messenger/internal/pkg/realtime"
	"github.com/jeogram/messenger/internal/pkg/response"
	"github.com/jeogram/messenger/internal/pkg/ws"
	"github.com/rs/zerolog/log"
)

// CallsHandler exposes call REST endpoints and a signaling websocket.
type CallsHandler struct {
	svc   *service.CallService
	chats *chatrepo.ChatRepository
	hub   realtime.Broadcaster
	jwt   *auth.JWT
}

func NewCallsHandler(svc *service.CallService, chats *chatrepo.ChatRepository, hub realtime.Broadcaster, jwt *auth.JWT) *CallsHandler {
	return &CallsHandler{svc: svc, chats: chats, hub: hub, jwt: jwt}
}

// RegisterRoutes mounts call endpoints.
//
//	@Summary	Start a call
//	@Tags		calls
//	@Accept		json
//	@Produce	json
//	@Param		body	body	startCallRequest	true	"call request"
//	@Success	201	{object}	response.APIResponse
//	@Router		/calls [post]
//	@Security	BearerAuth
func (h *CallsHandler) RegisterRoutes(r chi.Router) {
	r.With(middleware.JWTAuth(h.jwt)).Post("/calls", h.Start)
	r.With(middleware.JWTAuth(h.jwt)).Post("/calls/{id}/end", h.End)
	r.With(middleware.JWTAuth(h.jwt)).Get("/calls/ws", h.Signaling)
	r.With(middleware.JWTAuth(h.jwt)).Get("/calls/{id}", h.Get)
	r.With(middleware.JWTAuth(h.jwt)).Get("/calls/history", h.History)
	r.With(middleware.JWTAuth(h.jwt)).Get("/calls/ice-servers", h.ICEServers)
	r.With(middleware.JWTAuth(h.jwt)).Post("/calls/{id}/recording", h.SetRecording)
}

type startCallRequest struct {
	ChatID string `json:"chat_id" validate:"required"`
	Type   string `json:"type" validate:"required,oneof=audio video"`
}

// Start начинает звонок (audio/video) в чате.
// @Summary Начать звонок
// @Tags calls
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body startCallRequest true "чат и тип звонка"
// @Success 201 {object} response.APIResponse
// @Router /calls [post]
func (h *CallsHandler) Start(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	var req startCallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	call, err := h.svc.Start(r.Context(), userID, req.ChatID, domain.CallType(req.Type))
	if err != nil {
		if err == service.ErrForbidden {
			response.WriteError(w, http.StatusForbidden, "forbidden", err.Error())
			return
		}
		response.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	// Notify chat participants that a call is starting.
	participants, err := h.chats.Participants(r.Context(), req.ChatID)
	if err == nil {
		h.hub.SendToUsers(participants, ws.Outbound{
			Type:    "call.started",
			Payload: call,
		})
	}
	response.WriteCreated(w, call)
}

// End завершает звонок (только инициатор).
// @Summary Завершить звонок
// @Tags calls
// @Produce json
// @Security BearerAuth
// @Param id path string true "id звонка"
// @Success 200 {object} response.APIResponse
// @Router /calls/{id}/end [post]
func (h *CallsHandler) End(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	id := chi.URLParam(r, "id")
	if err := h.svc.End(r.Context(), id, userID); err != nil {
		if err == service.ErrForbidden {
			response.WriteError(w, http.StatusForbidden, "forbidden", err.Error())
			return
		}
		response.WriteError(w, http.StatusNotFound, "not_found", "call not found")
		return
	}
	response.WriteOK(w, map[string]string{"status": "ended"})
}

// Signaling ретранслирует WebRTC-сигналинг между участниками чата (WebSocket).
// @Summary WebRTC-сигналинг (WebSocket)
// @Tags calls
// @Produce json
// @Security BearerAuth
// @Success 101
// @Router /calls/ws [get]
func (h *CallsHandler) Signaling(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("call ws upgrade failed")
		return
	}
	defer conn.Close()

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var msg domain.SignalMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		ok, err := h.chats.IsParticipant(r.Context(), msg.ChatID, userID)
		if err != nil || !ok {
			continue
		}
		participants, err := h.chats.Participants(r.Context(), msg.ChatID)
		if err != nil {
			continue
		}
		recipients := participants
		if msg.To != "" {
			recipients = []string{msg.To}
		}
		// Never echo back to the sender.
		filtered := recipients[:0]
		for _, p := range recipients {
			if p != userID {
				filtered = append(filtered, p)
			}
		}
		h.hub.SendToUsers(filtered, ws.Outbound{
			Type: "call.signal",
			Payload: map[string]interface{}{
				"from":    userID,
				"type":    msg.Type,
				"chat_id": msg.ChatID,
				"data":    msg.Payload,
			},
		})
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// Get возвращает запись звонка по id.
//
//	@Summary	Получить звонок по id
//	@Tags		calls
//	@Produce	json
//	@Param		id		path		string	true	"id звонка"
//	@Success	200		{object}	response.APIResponse
//	@Router		/calls/{id} [get]
//	@Security	BearerAuth
func (h *CallsHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	call, err := h.svc.Get(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "not_found", "call not found")
		return
	}
	response.WriteOK(w, call)
}

// History возвращает историю звонков пользователя.
//
//	@Summary	История звонков
//	@Tags		calls
//	@Produce	json
//	@Param		limit	query		int		false	"лимит"
//	@Param		offset	query		int		false	"смещение"
//	@Success	200		{object}	response.APIResponse
//	@Router		/calls/history [get]
//	@Security	BearerAuth
func (h *CallsHandler) History(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	calls, err := h.svc.History(r.Context(), userID, limit, offset)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	response.WriteOK(w, calls)
}

// ICEServers возвращает STUN/TURN-конфигурацию для WebRTC.
// @Summary Получить STUN/TURN серверы для звонков
// @Tags calls
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /calls/ice-servers [get]
func (h *CallsHandler) ICEServers(w http.ResponseWriter, r *http.Request) {
	response.WriteOK(w, map[string]interface{}{
		"ice_servers":       h.svc.ICEServers(),
		"recording_enabled": h.svc.RecordingEnabled(),
	})
}

type setRecordingRequest struct {
	URL string `json:"url" validate:"required"`
}

// SetRecording прикрепляет ссылку на запись звонка (только инициатор).
// @Summary Сохранить запись звонка
// @Tags calls
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "id звонка"
// @Param body body setRecordingRequest true "url записи"
// @Success 200 {object} response.APIResponse
// @Router /calls/{id}/recording [post]
func (h *CallsHandler) SetRecording(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	id := chi.URLParam(r, "id")
	var req setRecordingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.URL == "" {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "url required")
		return
	}
	call, err := h.svc.SetRecording(r.Context(), id, userID, req.URL)
	if err != nil {
		if err == service.ErrForbidden {
			response.WriteError(w, http.StatusForbidden, "forbidden", err.Error())
			return
		}
		response.WriteError(w, http.StatusNotFound, "not_found", "call not found")
		return
	}
	response.WriteOK(w, call)
}
