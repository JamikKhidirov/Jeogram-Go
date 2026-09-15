package livekit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jeogram/messenger/internal/config"
	"github.com/jeogram/messenger/internal/modules/calls/domain"
	callrepo "github.com/jeogram/messenger/internal/modules/calls/repository"
	"github.com/jeogram/messenger/internal/modules/chat/repository"
	"github.com/jeogram/messenger/internal/pkg/webhook"
	"github.com/rs/zerolog/log"
)

type Service struct {
	apiKey    string
	apiSecret string
	serverURL string
	rtc       config.RTCConfig
	repo      *callrepo.CallRepository
	chats     *repository.ChatRepository
	webhooks  *webhook.Dispatcher
	httpClient *http.Client
}

func NewService(cfg config.LiveKitConfig, rtc config.RTCConfig, repo *callrepo.CallRepository, chats *repository.ChatRepository, webhooks *webhook.Dispatcher) (*Service, error) {
	s := &Service{
		apiKey:    cfg.APIKey,
		apiSecret: cfg.APISecret,
		serverURL: cfg.ServerURL,
		rtc:       rtc,
		repo:      repo,
		chats:     chats,
		webhooks:  webhooks,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	return s, nil
}

func (s *Service) CreateRoom(ctx context.Context, callID string, isGroup bool) (*Room, error) {
	roomName := fmt.Sprintf("%s_%s", s.getRoomPrefix(), callID)
	body := map[string]interface{}{
		"name":         roomName,
		"empty_timeout": 60,
		"record":       s.rtc.RecordingEnabled,
	}
	if isGroup {
		body["max_participants"] = 50
	}
	resp, err := s.apiPost(ctx, "/_api/rooms", body)
	if err != nil {
		return nil, fmt.Errorf("create room: %w", err)
	}
	defer resp.Body.Close()
	var room Room
	if err := json.NewDecoder(resp.Body).Decode(&room); err != nil {
		return nil, fmt.Errorf("decode room: %w", err)
	}
	log.Info().Str("room", roomName).Str("call", callID).Msg("livekit room created")
	return &room, nil
}

func (s *Service) DeleteRoom(ctx context.Context, roomName string) error {
	resp, err := s.apiDelete(ctx, "/_api/rooms/"+url.QueryEscape(roomName))
	if err != nil {
		return fmt.Errorf("delete room: %w", err)
	}
	defer resp.Body.Close()
	log.Info().Str("room", roomName).Msg("livekit room deleted")
	return nil
}

func (s *Service) GetRoom(ctx context.Context, roomName string) (*Room, error) {
	resp, err := s.apiGet(ctx, "/_api/rooms/"+url.QueryEscape(roomName))
	if err != nil {
		return nil, fmt.Errorf("get room: %w", err)
	}
	defer resp.Body.Close()
	var room Room
	if err := json.NewDecoder(resp.Body).Decode(&room); err != nil {
		return nil, err
	}
	return &room, nil
}

func (s *Service) ListRooms(ctx context.Context) ([]Room, error) {
	resp, err := s.apiGet(ctx, "/_api/rooms")
	if err != nil {
		return nil, fmt.Errorf("list rooms: %w", err)
	}
	defer resp.Body.Close()
	var rooms []Room
	if err := json.NewDecoder(resp.Body).Decode(&rooms); err != nil {
		return nil, err
	}
	return rooms, nil
}

func (s *Service) StartRecording(ctx context.Context, roomName string) error {
	resp, err := s.apiPost(ctx, "/_api/rooms/"+url.QueryEscape(roomName)+"/recordings", map[string]interface{}{})
	if err != nil {
		return fmt.Errorf("start recording: %w", err)
	}
	defer resp.Body.Close()
	log.Info().Str("room", roomName).Msg("livekit recording started")
	return nil
}

func (s *Service) StopRecording(ctx context.Context, roomName string) (string, error) {
	resp, err := s.apiDelete(ctx, "/_api/rooms/"+url.QueryEscape(roomName)+"/recordings")
	if err != nil {
		return "", fmt.Errorf("stop recording: %w", err)
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	url := ""
	if r, ok := result["url"].(string); ok {
		url = r
	}
	log.Info().Str("room", roomName).Str("url", url).Msg("livekit recording stopped")
	return url, nil
}

func (s *Service) GenerateAccessToken(roomName, userIdentity string) string {
	at := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iss":  s.apiKey,
		"sub":  userIdentity,
		"room": roomName,
		"video": jwt.MapClaims{
			"join":    true,
			"publish": true,
			"subscribe": true,
		},
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	})
	token, _ := at.SignedString([]byte(s.apiSecret))
	return token
}

func (s *Service) getRoomPrefix() string {
	if s.rtc.RecordingEnabled {
		return s.rtc.RecordingDir
	}
	return "jeogram"
}

func (s *Service) apiPost(ctx context.Context, path string, body map[string]interface{}) (*http.Response, error) {
	data, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, s.serverURL+path, bytes.NewReader(data))
	req.SetBasicAuth(s.apiKey, s.apiSecret)
	req.Header.Set("Content-Type", "application/json")
	return s.httpClient.Do(req)
}

func (s *Service) apiDelete(ctx context.Context, path string) (*http.Response, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodDelete, s.serverURL+path, nil)
	req.SetBasicAuth(s.apiKey, s.apiSecret)
	return s.httpClient.Do(req)
}

func (s *Service) apiGet(ctx context.Context, path string) (*http.Response, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.serverURL+path, nil)
	req.SetBasicAuth(s.apiKey, s.apiSecret)
	return s.httpClient.Do(req)
}

// Room represents a LiveKit room.
type Room struct {
	Name             string    `json:"name"`
	Sid              string    `json:"sid"`
	CreatedAt        time.Time `json:"created_at"`
	Active           bool      `json:"active"`
	EmptyTimeout     int       `json:"empty_timeout"`
	MaxParticipants  int       `json:"max_participants,omitempty"`
	ParticipantCount int       `json:"participant_count,omitempty"`
	Participants     []Participant `json:"participants,omitempty"`
}

// Participant represents a room participant.
type Participant struct {
	Identity string    `json:"identity"`
	SID      string    `json:"sid"`
	JoinedAt time.Time `json:"joined_at"`
	Tracks   int       `json:"tracks"`
}

func (s *Service) CreateCallWithLiveKit(ctx context.Context, initiator, chatID string, callType domain.CallType, isGroup bool) (*domain.Call, string, error) {
	ok, err := s.chats.IsParticipant(ctx, chatID, initiator)
	if err != nil {
		return nil, "", err
	}
	if !ok {
		return nil, "", domain.ErrForbidden
	}
	c := &domain.Call{ChatID: chatID, Initiator: initiator, Type: callType, Mode: domain.CallModePeer}
	if isGroup {
		c.Mode = domain.CallModeGroup
	}
	if err := s.repo.Create(ctx, c); err != nil {
		return nil, "", err
	}
	room, err := s.CreateRoom(ctx, c.ID, isGroup)
	if err != nil {
		log.Error().Err(err).Str("call", c.ID).Msg("failed to create livekit room")
	}
	roomName := ""
	if room != nil {
		roomName = room.Name
	}
	return c, roomName, nil
}

func (s *Service) EndCallWithLiveKit(ctx context.Context, callID, userID string) error {
	call, err := s.repo.Get(ctx, callID)
	if err != nil {
		return err
	}
	if call.Initiator != userID {
		return domain.ErrForbidden
	}
	err = s.DeleteRoom(ctx, fmt.Sprintf("%s_%s", s.getRoomPrefix(), callID))
	if err != nil {
		log.Error().Err(err).Str("call", callID).Msg("failed to delete livekit room")
	}
	return s.repo.End(ctx, callID)
}

func (s *Service) JoinRoom(ctx context.Context, callID, userID string) (string, string, error) {
	call, err := s.repo.Get(ctx, callID)
	if err != nil {
		return "", "", err
	}
	ok, err := s.chats.IsParticipant(ctx, call.ChatID, userID)
	if err != nil {
		return "", "", err
	}
	if !ok {
		return "", "", domain.ErrForbidden
	}
	roomName := fmt.Sprintf("%s_%s", s.getRoomPrefix(), callID)
	token := s.GenerateAccessToken(roomName, userID)
	return token, roomName, nil
}

func (s *Service) ListParticipants(ctx context.Context, roomName string) ([]Participant, error) {
	room, err := s.GetRoom(ctx, roomName)
	if err != nil {
		return nil, fmt.Errorf("get room: %w", err)
	}
	return room.Participants, nil
}

func (s *Service) ICEServers() []map[string]interface{} {
	if s.serverURL != "" {
		return []map[string]interface{}{
			{"urls": s.serverURL},
		}
	}
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

func (s *Service) RecordingEnabled() bool { return s.rtc.RecordingEnabled }
