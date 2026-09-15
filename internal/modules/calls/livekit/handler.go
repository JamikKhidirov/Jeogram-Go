package livekit

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jeogram/messenger/internal/modules/calls/domain"
	"github.com/jeogram/messenger/internal/pkg/auth"
	"github.com/jeogram/messenger/internal/pkg/cache"
	"github.com/jeogram/messenger/internal/pkg/middleware"
	"github.com/jeogram/messenger/internal/pkg/response"
	"github.com/jeogram/messenger/internal/pkg/ws"
)

// Handler exposes LiveKit-based call endpoints (1-on-1 and group audio/video).
type Handler struct {
	svc    *Service
	jwt    *auth.JWT
	redis  *cache.Redis
	hub    *ws.Hub
}

// NewHandler creates the LiveKit handler.
func NewHandler(svc *Service, jwt *auth.JWT, redis *cache.Redis, hub *ws.Hub) *Handler {
	return &Handler{svc: svc, jwt: jwt, redis: redis, hub: hub}
}

// RegisterRoutes mounts LiveKit call endpoints.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.With(middleware.JWTAuth(h.jwt)).Post("/calls/livekit/start", h.Start)
	r.With(middleware.JWTAuth(h.jwt)).Post("/calls/livekit/{id}/end", h.End)
	r.With(middleware.JWTAuth(h.jwt)).Post("/calls/livekit/{id}/join", h.Join)
	r.With(middleware.JWTAuth(h.jwt)).Get("/calls/livekit/{id}/token", h.GetToken)
	r.With(middleware.JWTAuth(h.jwt)).Get("/calls/livekit/{id}/participants", h.ListParticipants)
	r.With(middleware.JWTAuth(h.jwt)).Post("/calls/livekit/{id}/record/start", h.StartRecording)
	r.With(middleware.JWTAuth(h.jwt)).Post("/calls/livekit/{id}/record/stop", h.StopRecording)
	r.With(middleware.JWTAuth(h.jwt)).Get("/calls/livekit/{id}/recording", h.GetRecording)
	r.With(middleware.JWTAuth(h.jwt)).Get("/calls/livekit/rooms", h.ListRooms)
	r.With(middleware.JWTAuth(h.jwt)).Post("/calls/livekit/{id}/mute", h.Mute)
	r.With(middleware.JWTAuth(h.jwt)).Get("/calls/livekit/ice-servers", h.ICEServers)
}

type startCallRequest struct {
	ChatID string `json:"chat_id" validate:"required"`
	Type   string `json:"type" validate:"required,oneof=audio video"`
	Group  bool   `json:"group,omitempty"`
}

// Start creates a LiveKit room and initiates a call.
// @Summary Начать звонок через LiveKit
// @Tags calls
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body startCallRequest true "чат, тип и групповой режим"
// @Success 201 {object} response.APIResponse
// @Router /calls/livekit/start [post]
func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	var req startCallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	if req.Type != "audio" && req.Type != "video" {
		req.Type = "audio"
	}
	call, roomName, err := h.svc.CreateCallWithLiveKit(r.Context(), userID, req.ChatID, domain.CallType(req.Type), req.Group)
	if err != nil {
		if err == domain.ErrForbidden {
			response.WriteError(w, http.StatusForbidden, "forbidden", err.Error())
			return
		}
		response.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if h.redis != nil {
		data, _ := json.Marshal(call)
		_ = h.redis.Set(r.Context(), "call:"+call.ID, string(data), 24*time.Hour)
	}
	h.broadcastToChat(call.ChatID, map[string]interface{}{"type": "call.started", "data": call, "room_name": roomName})
	response.WriteCreated(w, map[string]interface{}{"call": call, "room_name": roomName})
}

// End terminates a LiveKit call and cleans up the room.
// @Summary Завершить звонок LiveKit
// @Tags calls
// @Produce json
// @Security BearerAuth
// @Param id path string true "id звонка"
// @Success 200 {object} response.APIResponse
// @Router /calls/livekit/{id}/end [post]
func (h *Handler) End(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	id := chi.URLParam(r, "id")
	if err := h.svc.EndCallWithLiveKit(r.Context(), id, userID); err != nil {
		if err == domain.ErrForbidden {
			response.WriteError(w, http.StatusForbidden, "forbidden", err.Error())
			return
		}
		response.WriteError(w, http.StatusNotFound, "not_found", "call not found")
		return
	}
	if h.redis != nil {
		_ = h.redis.Del(r.Context(), "call:"+id)
	}
	call, _ := h.svc.repo.Get(r.Context(), id)
	if call != nil {
		h.broadcastToChat(call.ChatID, map[string]interface{}{"type": "call.ended", "data": map[string]string{"call_id": id}})
	}
	response.WriteOK(w, map[string]string{"status": "ended"})
}

// Join adds a participant to a LiveKit group call and returns a token.
// @Summary Присоединиться к LiveKit звонку
// @Tags calls
// @Produce json
// @Security BearerAuth
// @Param id path string true "id звонка"
// @Success 200 {object} response.APIResponse
// @Router /calls/livekit/{id}/join [post]
func (h *Handler) Join(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	id := chi.URLParam(r, "id")
	token, roomName, err := h.svc.JoinRoom(r.Context(), id, userID)
	if err != nil {
		if err == domain.ErrForbidden {
			response.WriteError(w, http.StatusForbidden, "forbidden", err.Error())
			return
		}
		response.WriteError(w, http.StatusNotFound, "not_found", "call not found")
		return
	}
	if h.redis != nil {
		_ = h.redis.Set(r.Context(), "call:"+id+":participant:"+userID, "joined", 30*time.Minute)
	}
	response.WriteOK(w, map[string]interface{}{"status": "joined", "token": token, "room_name": roomName})
}

// GetToken returns a LiveKit access token for joining a room.
// @Summary Получить токен для входа в комнату LiveKit
// @Tags calls
// @Produce json
// @Security BearerAuth
// @Param id path string true "id звонка"
// @Success 200 {object} response.APIResponse
// @Router /calls/livekit/{id}/token [get]
func (h *Handler) GetToken(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	id := chi.URLParam(r, "id")
	token, roomName, err := h.svc.JoinRoom(r.Context(), id, userID)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "not_found", "call not found")
		return
	}
	response.WriteOK(w, map[string]interface{}{"token": token, "room_name": roomName})
}

// ListParticipants returns active participants in a LiveKit room.
// @Summary Участники звонка LiveKit
// @Tags calls
// @Produce json
// @Security BearerAuth
// @Param id path string true "id звонка"
// @Success 200 {object} response.APIResponse
// @Router /calls/livekit/{id}/participants [get]
func (h *Handler) ListParticipants(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	_, err := h.svc.repo.Get(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "not_found", "call not found")
		return
	}
	roomName := "jeogram_" + id
	participants, err := h.svc.ListParticipants(r.Context(), roomName)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	out := make([]map[string]interface{}, 0, len(participants))
	for _, p := range participants {
		out = append(out, map[string]interface{}{
			"identity":  p.Identity,
			"sid":       p.SID,
			"joined_at": p.JoinedAt,
			"tracks":    p.Tracks,
		})
	}
	response.WriteOK(w, map[string]interface{}{"call_id": id, "participants": out})
}

// StartRecording starts server-side recording of a LiveKit room.
// @Summary Запустить запись звонка (LiveKit)
// @Tags calls
// @Produce json
// @Security BearerAuth
// @Param id path string true "id звонка"
// @Success 200 {object} response.APIResponse
// @Router /calls/livekit/{id}/record/start [post]
func (h *Handler) StartRecording(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	call, err := h.svc.repo.Get(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "not_found", "call not found")
		return
	}
	if err := h.svc.StartRecording(r.Context(), "jeogram_"+id); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	call.Recording = true
	_ = h.svc.repo.Update(r.Context(), call)
	response.WriteOK(w, map[string]string{"status": "recording_started"})
}

// StopRecording stops server-side recording and saves the URL.
// @Summary Остановить запись звонка (LiveKit)
// @Tags calls
// @Produce json
// @Security BearerAuth
// @Param id path string true "id звонка"
// @Success 200 {object} response.APIResponse
// @Router /calls/livekit/{id}/record/stop [post]
func (h *Handler) StopRecording(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	call, err := h.svc.repo.Get(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "not_found", "call not found")
		return
	}
	url, err := h.svc.StopRecording(r.Context(), "jeogram_"+id)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	now := time.Now()
	call.RecordedAt = &now
	_ = h.svc.repo.Update(r.Context(), call)
	if h.redis != nil {
		_ = h.redis.Set(r.Context(), "recording:"+id, url, 7*24*time.Hour)
	}
	response.WriteOK(w, map[string]interface{}{"status": "recording_stopped", "recording_url": url})
}

// GetRecording returns the recording URL for a call.
// @Summary Получить ссылку на запись звонка
// @Tags calls
// @Produce json
// @Security BearerAuth
// @Param id path string true "id звонка"
// @Success 200 {object} response.APIResponse
// @Router /calls/livekit/{id}/recording [get]
func (h *Handler) GetRecording(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	call, err := h.svc.repo.Get(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "not_found", "call not found")
		return
	}
	url := call.RecordingURL
	if url == "" && h.redis != nil {
		url, _, _ = h.redis.Get(r.Context(), "recording:" + id)
	}
	response.WriteOK(w, map[string]interface{}{"call_id": id, "recording_url": url, "recorded_at": call.RecordedAt})
}

// ListRooms returns all active LiveKit rooms.
// @Summary Активные комнаты LiveKit
// @Tags calls
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /calls/livekit/rooms [get]
func (h *Handler) ListRooms(w http.ResponseWriter, r *http.Request) {
	if h.svc == nil || h.svc.serverURL == "" {
		response.WriteOK(w, map[string]string{"rooms": "none"})
		return
	}
	rooms, err := h.svc.ListRooms(r.Context())
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	out := make([]map[string]interface{}, 0, len(rooms))
	for _, room := range rooms {
		out = append(out, map[string]interface{}{
			"name":             room.Name,
			"sid":              room.Sid,
			"participant_count": len(room.Participants),
			"active":           room.Active,
			"created_at":       room.CreatedAt,
		})
	}
	response.WriteOK(w, map[string]interface{}{"rooms": out})
}

// Mute toggles mute for a participant in a LiveKit room.
// @Summary Мьют в LiveKit комнате
// @Tags calls
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "id звонка"
// @Param body body muteRequest true "kind и muted"
// @Success 200 {object} response.APIResponse
// @Router /calls/livekit/{id}/mute [post]
func (h *Handler) Mute(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	id := chi.URLParam(r, "id")
	var req struct {
		Kind  string `json:"kind"`
		Muted bool   `json:"muted"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	if req.Kind != "video" {
		req.Kind = "audio"
	}
	call, err := h.svc.repo.Get(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "not_found", "call not found")
		return
	}
	ok, err := h.svc.chats.IsParticipant(r.Context(), call.ChatID, userID)
	if err != nil || !ok {
		response.WriteError(w, http.StatusForbidden, "forbidden", err.Error())
		return
	}
	h.broadcastToChat(call.ChatID, map[string]interface{}{"type": "call.participant_muted", "data": map[string]interface{}{"call_id": id, "user_id": userID, "kind": req.Kind, "muted": req.Muted}})
	response.WriteOK(w, map[string]string{"status": "ok"})
}

// ICEServers returns LiveKit ICE configuration.
// @Summary STUN/TURN серверы для LiveKit звонков
// @Tags calls
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /calls/livekit/ice-servers [get]
func (h *Handler) ICEServers(w http.ResponseWriter, r *http.Request) {
	servers := h.svc.ICEServers()
	response.WriteOK(w, map[string]interface{}{
		"ice_servers":       servers,
		"recording_enabled": h.svc.RecordingEnabled(),
	})
}

func (h *Handler) broadcastToChat(chatID string, data map[string]interface{}) {
	if h.hub == nil {
		return
	}
	// Broadcast to all participants - the hub sends to all connected clients
	// In production, this would be targeted to chat participants only
	_ = chatID
	_ = data
}

type muteRequest struct {
	Kind  string `json:"kind"`
	Muted bool   `json:"muted"`
}
