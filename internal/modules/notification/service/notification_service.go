package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	authrepo "github.com/jeogram/messenger/internal/modules/auth/repository"
	chatrepo "github.com/jeogram/messenger/internal/modules/chat/repository"
	"github.com/jeogram/messenger/internal/modules/notification/domain"
	devrepo "github.com/jeogram/messenger/internal/modules/notification/repository"
	"github.com/jeogram/messenger/internal/pkg/events"
	"github.com/jeogram/messenger/internal/pkg/ws"
	"github.com/rs/zerolog/log"
)

// NotificationService consumes message events and fans them out to push + realtime.
type NotificationService struct {
	devices     *devrepo.DeviceRepository
	notifs      *devrepo.NotificationRepository
	users       *authrepo.UserRepository
	chats       *chatrepo.ChatRepository
	push        *PushService
	hub         *ws.Hub
	producer    *events.Producer
	notifyTopic string
}

func NewNotificationService(
	devices *devrepo.DeviceRepository,
	notifs *devrepo.NotificationRepository,
	users *authrepo.UserRepository,
	chats *chatrepo.ChatRepository,
	push *PushService,
	hub *ws.Hub,
	producer *events.Producer,
	notifyTopic string,
) *NotificationService {
	return &NotificationService{
		devices:     devices,
		notifs:      notifs,
		users:       users,
		chats:       chats,
		push:        push,
		hub:         hub,
		producer:    producer,
		notifyTopic: notifyTopic,
	}
}

// HandleMessageCreated delivers a realtime + push notification to every
// participant of the chat except the sender.
func (s *NotificationService) HandleMessageCreated(ctx context.Context, ev events.MessageCreatedEvent) {
	participants, err := s.chats.Participants(ctx, ev.ChatID)
	if err != nil {
		log.Error().Err(err).Str("chat", ev.ChatID).Msg("cannot load participants")
		return
	}
	sender, err := s.users.GetByID(ctx, ev.SenderID)
	if err != nil {
		log.Error().Err(err).Msg("cannot load sender")
		return
	}

	for _, uid := range participants {
		if uid == ev.SenderID {
			continue
		}
		body := previewFor(ev)
		title := sender.DisplayName

		// Persist in-app notification.
		n := &domain.Notification{
			ID:        uuid.NewString(),
			UserID:    uid,
			ChatID:    ev.ChatID,
			MessageID: ev.MessageID,
			Title:     title,
			Body:      body,
			Type:      ev.Type,
			Read:      false,
		}
		if err := s.notifs.Create(ctx, n); err != nil {
			log.Error().Err(err).Msg("could not store notification")
		}

		// Real-time delivery via websocket hub.
		s.hub.SendToUsers([]string{uid}, ws.Outbound{
			Type: "message.new",
			Payload: map[string]interface{}{
				"chat_id":    ev.ChatID,
				"message_id": ev.MessageID,
				"sender_id":  ev.SenderID,
				"type":       ev.Type,
				"preview":    body,
			},
		})

		// Push notification to registered devices.
		devices, err := s.devices.TokensForUser(ctx, uid)
		if err != nil {
			log.Error().Err(err).Msg("cannot load devices")
			continue
		}
		s.push.Notify(ctx, devices, domain.PushPayload{
			UserID:    uid,
			Title:     title,
			Body:      body,
			ChatID:    ev.ChatID,
			MessageID: ev.MessageID,
			Type:      ev.Type,
		})

		// Forward to an external notification topic for other consumers.
		if s.producer != nil {
			if b, err := json.Marshal(n); err == nil {
				_ = s.producer.Publish(ctx, s.notifyTopic, uid, b)
			}
		}
	}
}

// RegisterDevice stores (or updates) a push token for a user.
func (s *NotificationService) RegisterDevice(ctx context.Context, userID string, platform domain.Platform, token string) {
	d := &domain.DeviceToken{
		UserID:    userID,
		Platform:  platform,
		Token:     token,
		CreatedAt: time.Now(),
	}
	if err := s.devices.Upsert(ctx, d); err != nil {
		log.Error().Err(err).Msg("could not register device")
	}
}

// List returns a user's in-app notifications.
func (s *NotificationService) List(ctx context.Context, userID string) ([]domain.PublicNotification, error) {
	ns, err := s.notifs.List(ctx, userID, 50, 0)
	if err != nil {
		return nil, err
	}
	out := make([]domain.PublicNotification, 0, len(ns))
	for i := range ns {
		out = append(out, domain.PublicNotification{
			ID:        ns[i].ID,
			ChatID:    ns[i].ChatID,
			MessageID: ns[i].MessageID,
			Title:     ns[i].Title,
			Body:      ns[i].Body,
			Type:      ns[i].Type,
			Read:      ns[i].Read,
			CreatedAt: ns[i].CreatedAt.Format(time.RFC3339),
		})
	}
	return out, nil
}

// MarkAllRead marks every notification for a user as read.
func (s *NotificationService) MarkAllRead(ctx context.Context, userID string) error {
	return s.notifs.MarkAllRead(ctx, userID)
}

// Run consumes the message.created topic until the context is cancelled.
func (s *NotificationService) Run(ctx context.Context, consumer *events.Consumer) {
	log.Info().Msg("notification consumer started")
	for {
		msg, err := consumer.Read(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Error().Err(err).Msg("kafka read error")
			continue
		}
		var ev events.MessageCreatedEvent
		if err := json.Unmarshal(msg.Value, &ev); err != nil {
			log.Error().Err(err).Msg("could not unmarshal event")
			continue
		}
		s.HandleMessageCreated(ctx, ev)
	}
}

func previewFor(ev events.MessageCreatedEvent) string {
	switch ev.Type {
	case "text":
		return ev.Text
	case "voice":
		return "🎤 Voice message"
	case "image":
		return "📷 Photo"
	default:
		return "New message"
	}
}
