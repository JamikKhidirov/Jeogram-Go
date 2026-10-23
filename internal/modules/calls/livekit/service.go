package livekit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jeogram/messenger/internal/config"
	"github.com/jeogram/messenger/internal/modules/calls/domain"
	callrepo "github.com/jeogram/messenger/internal/modules/calls/repository"
	"github.com/jeogram/messenger/internal/modules/chat/repository"
	"github.com/jeogram/messenger/internal/pkg/webhook"
	"github.com/rs/zerolog/log"
)

// LiveKit >= 1.3 exposes its management API over Twirp (JSON), аутентификация —
// JWT, подписанный API-секретом, с грантами в поле "video".
const (
	twirpRoomService   = "/twirp/livekit.RoomService/"
	twirpEgressService = "/twirp/livekit.EgressService/"
)

type Service struct {
	apiKey     string
	apiSecret  string
	serverURL  string
	publicURL  string
	rtc        config.RTCConfig
	repo       *callrepo.CallRepository
	chats      *repository.ChatRepository
	webhooks   *webhook.Dispatcher
	httpClient *http.Client
}

func NewService(cfg config.LiveKitConfig, rtc config.RTCConfig, repo *callrepo.CallRepository, chats *repository.ChatRepository, webhooks *webhook.Dispatcher) (*Service, error) {
	apiBase := strings.TrimSuffix(cfg.ServerURL, "/")
	if strings.HasPrefix(apiBase, "ws://") {
		apiBase = "http://" + strings.TrimPrefix(apiBase, "ws://")
	} else if strings.HasPrefix(apiBase, "wss://") {
		apiBase = "https://" + strings.TrimPrefix(apiBase, "wss://")
	}
	s := &Service{
		apiKey:     cfg.APIKey,
		apiSecret:  cfg.APISecret,
		serverURL:  apiBase,
		publicURL:  cfg.PublicURL,
		rtc:        rtc,
		repo:       repo,
		chats:      chats,
		webhooks:   webhooks,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	return s, nil
}

// ClientURL returns the public LiveKit address clients must connect to
// (wss://host — SDK сам дописывает /rtc?access_token=...).
func (s *Service) ClientURL() string {
	if s.publicURL != "" {
		return s.publicURL
	}
	return s.serverURL
}

// apiToken builds a short-lived JWT for the LiveKit management API.
// Гранты определяют, какие операции разрешены (см. LiveKit Ensure*Permission):
//   - roomCreate — создание/удаление комнат
//   - roomList   — список комнат
//   - roomAdmin  + room=<имя комнаты> — операции с участниками комнаты
//   - roomRecord — запись звонка
func (s *Service) apiToken(grants map[string]interface{}) string {
	now := time.Now()
	claims := jwt.MapClaims{
		"iss":   s.apiKey,
		"sub":   s.apiKey,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
		"video": grants,
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.apiSecret))
	if err != nil {
		return ""
	}
	return tok
}

// apiCall performs a Twirp call against the LiveKit server.
func (s *Service) apiCall(ctx context.Context, service, method string, body map[string]interface{}, grants map[string]interface{}) ([]byte, error) {
	payload := []byte("{}")
	if body != nil {
		payload, _ = json.Marshal(body)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.serverURL+service+method, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiToken(grants))
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("livekit %s: %s (%d)", method, string(data), resp.StatusCode)
	}
	return data, nil
}

func (s *Service) CreateRoom(ctx context.Context, callID string, isGroup bool) (*Room, error) {
	roomName := fmt.Sprintf("%s_%s", s.getRoomPrefix(), callID)
	body := map[string]interface{}{
		"name":          roomName,
		"empty_timeout": 60,
	}
	if isGroup {
		body["max_participants"] = 50
	}
	data, err := s.apiCall(ctx, twirpRoomService, "CreateRoom", body, map[string]interface{}{"roomCreate": true})
	if err != nil {
		return nil, fmt.Errorf("create room: %w", err)
	}
	var room Room
	if err := json.Unmarshal(data, &room); err != nil {
		return nil, fmt.Errorf("decode room: %w", err)
	}
	log.Info().Str("room", roomName).Str("call", callID).Msg("livekit room created")
	return &room, nil
}

func (s *Service) DeleteRoom(ctx context.Context, roomName string) error {
	_, err := s.apiCall(ctx, twirpRoomService, "DeleteRoom", map[string]interface{}{"room": roomName},
		map[string]interface{}{"roomCreate": true})
	if err != nil {
		return fmt.Errorf("delete room: %w", err)
	}
	log.Info().Str("room", roomName).Msg("livekit room deleted")
	return nil
}

func (s *Service) GetRoom(ctx context.Context, roomName string) (*Room, error) {
	rooms, err := s.ListRooms(ctx, roomName)
	if err != nil {
		return nil, err
	}
	for i := range rooms {
		if rooms[i].Name == roomName {
			return &rooms[i], nil
		}
	}
	return nil, fmt.Errorf("room %s not found", roomName)
}

// ListRooms returns all active rooms. Если roomName задан — только его.
func (s *Service) ListRooms(ctx context.Context, roomName ...string) ([]Room, error) {
	body := map[string]interface{}{}
	if len(roomName) > 0 && roomName[0] != "" {
		body["names"] = []string{roomName[0]}
	}
	data, err := s.apiCall(ctx, twirpRoomService, "ListRooms", body, map[string]interface{}{"roomList": true})
	if err != nil {
		return nil, fmt.Errorf("list rooms: %w", err)
	}
	var res struct {
		Rooms []Room `json:"rooms"`
	}
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, fmt.Errorf("decode rooms: %w", err)
	}
	return res.Rooms, nil
}

// ListParticipants returns the participants of a room.
func (s *Service) ListParticipants(ctx context.Context, roomName string) ([]Participant, error) {
	data, err := s.apiCall(ctx, twirpRoomService, "ListParticipants", map[string]interface{}{"room": roomName},
		map[string]interface{}{"roomAdmin": true, "room": roomName})
	if err != nil {
		return nil, fmt.Errorf("list participants: %w", err)
	}
	var res struct {
		Participants []Participant `json:"participants"`
	}
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, fmt.Errorf("decode participants: %w", err)
	}
	return res.Participants, nil
}

// StartRecording starts a composite (room) recording through the egress service.
// Требует развёрнутого livekit-egress, иначе LiveKit вернёт ошибку.
func (s *Service) StartRecording(ctx context.Context, roomName string) error {
	body := map[string]interface{}{
		"room_name": roomName,
		"layout":    "speaker",
		"file_outputs": []map[string]interface{}{
			{
				"file_type": "MP4",
				"filepath":  fmt.Sprintf("%s/%s.mp4", strings.TrimSuffix(s.rtc.RecordingDir, "/"), roomName),
			},
		},
	}
	if _, err := s.apiCall(ctx, twirpEgressService, "StartRoomCompositeEgress", body,
		map[string]interface{}{"roomRecord": true}); err != nil {
		if strings.Contains(err.Error(), "404") {
			return fmt.Errorf("запись звонка недоступна: в LiveKit не подключён сервис Egress " +
				"(запустите livekit-egress или отключите RTC_RECORDING_ENABLED)")
		}
		return fmt.Errorf("start recording: %w", err)
	}
	log.Info().Str("room", roomName).Msg("livekit recording started")
	return nil
}

// StopRecording stops the room recording and returns the resulting file URL.
func (s *Service) StopRecording(ctx context.Context, roomName string) (string, error) {
	data, err := s.apiCall(ctx, twirpEgressService, "ListEgress", map[string]interface{}{"room_name": roomName},
		map[string]interface{}{"roomRecord": true})
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			return "", fmt.Errorf("запись звонка недоступна: в LiveKit не подключён сервис Egress " +
				"(запустите livekit-egress или отключите RTC_RECORDING_ENABLED)")
		}
		return "", fmt.Errorf("list egress: %w", err)
	}
	var list struct {
		Items []struct {
			EgressID string `json:"egress_id"`
			RoomName string `json:"room_name"`
			Status   string `json:"status"`
			File     struct {
				Filename string `json:"filename"`
				Location string `json:"location"`
			} `json:"file"`
		} `json:"items"`
	}
	_ = json.Unmarshal(data, &list)
	target := ""
	for _, item := range list.Items {
		if item.RoomName == roomName {
			target = item.EgressID
			break
		}
	}
	if target == "" {
		return "", fmt.Errorf("no active recording for room %s", roomName)
	}
	stop, err := s.apiCall(ctx, twirpEgressService, "StopEgress", map[string]interface{}{"egress_id": target},
		map[string]interface{}{"roomRecord": true})
	if err != nil {
		return "", fmt.Errorf("stop recording: %w", err)
	}
	var stopped struct {
		Result struct {
			File struct {
				Location string `json:"location"`
			} `json:"file"`
		} `json:"result"`
	}
	_ = json.Unmarshal(stop, &stopped)
	log.Info().Str("room", roomName).Str("url", stopped.Result.File.Location).Msg("livekit recording stopped")
	return stopped.Result.File.Location, nil
}

// GenerateAccessToken builds a client access token for a room.
func (s *Service) GenerateAccessToken(roomName, userIdentity string) string {
	at := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iss":  s.apiKey,
		"sub":  userIdentity,
		"room": roomName,
		"video": jwt.MapClaims{
			"join":      true,
			"publish":   true,
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

// ICEServers returns STUN/TURN servers for the WebRTC media path.
func (s *Service) ICEServers() []map[string]interface{} {
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

// Room represents a LiveKit room (Twirp JSON).
type Room struct {
	SID              string `json:"sid"`
	Name             string `json:"name"`
	CreationTime     string `json:"creation_time"`
	CreationTimeMs   string `json:"creation_time_ms"`
	EmptyTimeout     int    `json:"empty_timeout"`
	DepartureTimeout int    `json:"departure_timeout"`
	MaxParticipants  int    `json:"max_participants"`
	NumParticipants  int    `json:"num_participants"`
	Metadata         string `json:"metadata"`
	ActiveRecording  bool   `json:"active_recording"`
}

// Participant represents a room participant (Twirp JSON).
type Participant struct {
	Identity string      `json:"identity"`
	SID      string      `json:"sid"`
	State    string      `json:"state"`
	JoinedAt string      `json:"joined_at"`
	Tracks   []TrackInfo `json:"tracks"`
}

// TrackInfo describes a published track of a participant.
type TrackInfo struct {
	SID   string `json:"sid"`
	Type  string `json:"type"`
	Muted bool   `json:"muted"`
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
	roomName := fmt.Sprintf("%s_%s", s.getRoomPrefix(), c.ID)
	if room != nil && room.Name != "" {
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
	if err := s.DeleteRoom(ctx, fmt.Sprintf("%s_%s", s.getRoomPrefix(), callID)); err != nil {
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

func (s *Service) RecordingEnabled() bool { return s.rtc.RecordingEnabled }
