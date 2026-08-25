package service

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/jeogram/messenger/internal/config"
	"github.com/jeogram/messenger/internal/modules/calls/domain"
	"github.com/jeogram/messenger/internal/modules/calls/repository"
	chatrepo "github.com/jeogram/messenger/internal/modules/chat/repository"
	"github.com/jeogram/messenger/internal/pkg/webhook"
)

// ErrForbidden indicates the user may not act on the call.
var ErrForbidden = errors.New("forbidden")

// CallService manages call lifecycle.
type CallService struct {
	repo     *repository.CallRepository
	chats    *chatrepo.ChatRepository
	rtc      config.RTCConfig
	webhooks *webhook.Dispatcher

	mu    sync.Mutex
	mutes map[string]map[string]bool // callID -> (userID -> muted)
}

func NewCallService(repo *repository.CallRepository, chats *chatrepo.ChatRepository, rtc config.RTCConfig, webhooks *webhook.Dispatcher) *CallService {
	return &CallService{repo: repo, chats: chats, rtc: rtc, webhooks: webhooks}
}

// ICEServers returns the STUN/TURN configuration for WebRTC negotiation.
func (s *CallService) ICEServers() []map[string]interface{} {
	servers := make([]map[string]interface{}, 0)
	for _, srv := range s.rtc.STUNServers {
		servers = append(servers, map[string]interface{}{"urls": srv})
	}
	for _, srv := range s.rtc.TURNServers {
		servers = append(servers, map[string]interface{}{
			"urls":       srv,
			"username":   s.rtc.TURNUser,
			"credential": s.rtc.TURNPassword,
		})
	}
	if len(servers) == 0 {
		servers = append(servers, map[string]interface{}{"urls": "stun:stun.l.google.com:19302"})
	}
	return servers
}

// RecordingEnabled reports whether call recording is enabled.
func (s *CallService) RecordingEnabled() bool { return s.rtc.RecordingEnabled }

// SetRecording attaches a recording url to a call (only initiator).
func (s *CallService) SetRecording(ctx context.Context, id, userID, url string) (*domain.Call, error) {
	call, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if call.Initiator != userID {
		return nil, ErrForbidden
	}
	now := time.Now()
	call.RecordingURL = url
	call.RecordedAt = &now
	if err := s.repo.Update(ctx, call); err != nil {
		return nil, err
	}
	return call, nil
}

// Start records a new call if the initiator belongs to the chat.
// mode задаёт peer-to-peer или групповой (SFU-ready) режим звонка.
func (s *CallService) Start(ctx context.Context, initiator, chatID string, callType domain.CallType, mode domain.CallMode) (*domain.Call, error) {
	ok, err := s.chats.IsParticipant(ctx, chatID, initiator)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrForbidden
	}
	if mode == "" {
		mode = domain.CallModePeer
	}
	c := &domain.Call{ChatID: chatID, Initiator: initiator, Type: callType, Mode: mode}
	if err := s.repo.Create(ctx, c); err != nil {
		return nil, err
	}
	if s.webhooks != nil {
		s.webhooks.Send("call.started", map[string]interface{}{
			"call_id":   c.ID,
			"chat_id":   c.ChatID,
			"initiator": c.Initiator,
			"type":      string(c.Type),
			"mode":      string(c.Mode),
		})
	}
	return c, nil
}

// End terminates a call (only the initiator may end it).
func (s *CallService) End(ctx context.Context, id, userID string) error {
	call, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if call.Initiator != userID {
		return ErrForbidden
	}
	return s.repo.End(ctx, id)
}

// Get возвращает запись звонка по id.
func (s *CallService) Get(ctx context.Context, id string) (*domain.Call, error) {
	return s.repo.Get(ctx, id)
}

// History возвращает историю звонков пользователя.
func (s *CallService) History(ctx context.Context, userID string, limit, offset int) ([]domain.Call, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.repo.ListByUser(ctx, userID, limit, offset)
}

// ListActive возвращает все активные звонки (для отображения в UI).
func (s *CallService) ListActive(ctx context.Context) ([]domain.Call, error) {
	return s.repo.ListActive(ctx)
}

// SetMute запоминает состояние микрофона/камеры участника во время звонка.
// Возвращает ошибку, если пользователь не участвует в чате звонка.
func (s *CallService) SetMute(ctx context.Context, callID, userID, kind string, muted bool) error {
	call, err := s.repo.Get(ctx, callID)
	if err != nil {
		return err
	}
	ok, err := s.chats.IsParticipant(ctx, call.ChatID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	s.mu.Lock()
	if s.mutes == nil {
		s.mutes = make(map[string]map[string]bool)
	}
	if s.mutes[callID] == nil {
		s.mutes[callID] = make(map[string]bool)
	}
	key := userID + ":" + kind
	s.mutes[callID][key] = muted
	s.mu.Unlock()
	return nil
}

// SetRecordingState включает/выключает запись звонка (только инициатор).
func (s *CallService) SetRecordingState(ctx context.Context, callID, userID string, recording bool) (*domain.Call, error) {
	call, err := s.repo.Get(ctx, callID)
	if err != nil {
		return nil, err
	}
	if call.Initiator != userID {
		return nil, ErrForbidden
	}
	call.Recording = recording
	if err := s.repo.Update(ctx, call); err != nil {
		return nil, err
	}
	return call, nil
}

// Join добавляет участника в групповой звонок (проверка членства в чате).
func (s *CallService) Join(ctx context.Context, callID, userID string) error {
	call, err := s.repo.Get(ctx, callID)
	if err != nil {
		return err
	}
	ok, err := s.chats.IsParticipant(ctx, call.ChatID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	return nil
}
