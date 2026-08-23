package service

import (
	"context"
	"errors"
	"time"

	"github.com/jeogram/messenger/internal/config"
	chatrepo "github.com/jeogram/messenger/internal/modules/chat/repository"
	"github.com/jeogram/messenger/internal/modules/message/domain"
	msgrepo "github.com/jeogram/messenger/internal/modules/message/repository"
	"github.com/jeogram/messenger/internal/pkg/events"
	"github.com/jeogram/messenger/internal/pkg/realtime"
	"github.com/jeogram/messenger/internal/pkg/ws"
	"github.com/rs/zerolog/log"
)

// ErrForbidden indicates the user may not act on the message/chat.
var ErrForbidden = errors.New("forbidden")

// MessageService implements messaging use cases and publishes domain events.
type MessageService struct {
	repo   *msgrepo.MessageRepository
	chats  *chatrepo.ChatRepository
	kafka  *events.Producer
	topics config.KafkaConfig
	hub    realtime.Broadcaster
}

func NewMessageService(repo *msgrepo.MessageRepository, chats *chatrepo.ChatRepository, kafka *events.Producer, topics config.KafkaConfig, hub realtime.Broadcaster) *MessageService {
	return &MessageService{repo: repo, chats: chats, kafka: kafka, topics: topics, hub: hub}
}

// Send stores a message and publishes a MessageCreated event to Kafka.
func (s *MessageService) Send(ctx context.Context, senderID string, req domain.SendMessageRequest) (*domain.PublicMessage, error) {
	ok, err := s.chats.IsParticipant(ctx, req.ChatID, senderID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrForbidden
	}
	if req.Type == string(domain.TypeText) && req.Text == "" {
		return nil, errors.New("text is required for text messages")
	}
	if (req.Type == string(domain.TypeVoice) || req.Type == string(domain.TypeImage)) && req.MediaURL == "" {
		return nil, errors.New("media_url is required for media messages")
	}

	msg := &domain.Message{
		ChatID:   req.ChatID,
		SenderID: senderID,
		Type:     domain.MessageType(req.Type),
		Text:     req.Text,
		MediaURL: req.MediaURL,
	}
	if req.ReplyTo != "" {
		msg.ReplyToID = &req.ReplyTo
	}
	if err := s.repo.Create(ctx, msg); err != nil {
		return nil, err
	}
	if err := s.chats.Touch(ctx, req.ChatID); err != nil {
		log.Warn().Err(err).Msg("could not touch chat")
	}

	event := events.MessageCreatedEvent{
		MessageID: msg.ID,
		ChatID:    msg.ChatID,
		SenderID:  msg.SenderID,
		Type:      string(msg.Type),
		Text:      msg.Text,
		MediaURL:  msg.MediaURL,
		CreatedAt: msg.CreatedAt,
	}
	if s.kafka != nil {
		if err := s.kafka.Publish(ctx, s.topics.MessageTopic, msg.ChatID, event); err != nil {
			log.Error().Err(err).Msg("failed to publish message.created event")
		}
	}

	// Realtime-доставка всем участникам чата через WebSocket-хаб.
	// Публикация в Kafka используется для внешних консьюмеров (search/analytics),
	// а мгновенная доставка в WS делается здесь, чтобы клиенты получали
	// сообщения в реальном времени независимо от наличия Kafka.
	if s.hub != nil {
		if pm, err := s.toPublic(ctx, msg); err == nil {
			participants, perr := s.chats.Participants(ctx, msg.ChatID)
			if perr == nil {
				s.hub.SendToUsers(participants, ws.Outbound{Type: "message.new", Payload: pm})
			}
		}
	}

	return s.toPublic(ctx, msg)
}

// List returns messages for a chat (access-checked), с реакциями/закрепами/ответами.
func (s *MessageService) List(ctx context.Context, chatID, userID string, limit, offset int) ([]domain.PublicMessage, error) {
	ok, err := s.chats.IsParticipant(ctx, chatID, userID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrForbidden
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	msgs, err := s.repo.List(ctx, chatID, limit, offset)
	if err != nil {
		return nil, err
	}
	return s.toPublicBulk(ctx, msgs)
}

// Edit updates the text of a message (sender only).
func (s *MessageService) Edit(ctx context.Context, messageID, userID, text string) (*domain.PublicMessage, error) {
	msg, err := s.repo.Get(ctx, messageID)
	if err != nil {
		return nil, err
	}
	if msg.SenderID != userID {
		return nil, ErrForbidden
	}
	now := time.Now()
	msg.Text = text
	msg.EditedAt = &now
	if err := s.repo.Update(ctx, msg); err != nil {
		return nil, err
	}
	return s.toPublic(ctx, msg)
}

// Delete soft-deletes a message (sender only).
func (s *MessageService) Delete(ctx context.Context, messageID, userID string) error {
	msg, err := s.repo.Get(ctx, messageID)
	if err != nil {
		return err
	}
	if msg.SenderID != userID {
		return ErrForbidden
	}
	now := time.Now()
	msg.DeletedAt = &now
	return s.repo.Update(ctx, msg)
}

// MarkRead records read receipts for the given messages.
func (s *MessageService) MarkRead(ctx context.Context, chatID, userID string, ids []string) error {
	ok, err := s.chats.IsParticipant(ctx, chatID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	return s.repo.SaveReadReceipts(ctx, userID, ids)
}

// UnreadCount returns how many messages the user has not read in a chat.
func (s *MessageService) UnreadCount(ctx context.Context, chatID, userID string) (int64, error) {
	return s.repo.CountUnread(ctx, chatID, userID)
}

// MarkReadOne marks a single message as read by the user (resolves chat automatically).
func (s *MessageService) MarkReadOne(ctx context.Context, userID, messageID string) error {
	msg, err := s.repo.Get(ctx, messageID)
	if err != nil {
		return err
	}
	return s.MarkRead(ctx, msg.ChatID, userID, []string{messageID})
}

// ListPinned возвращает закреплённые сообщения чата.
func (s *MessageService) ListPinned(ctx context.Context, userID, chatID string, limit int) ([]domain.PublicMessage, error) {
	ok, err := s.chats.IsParticipant(ctx, chatID, userID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrForbidden
	}
	msgs, err := s.repo.ListPinned(ctx, chatID, limit)
	if err != nil {
		return nil, err
	}
	return s.toPublicBulk(ctx, msgs)
}

// ClearHistory мягко удаляет всю историю сообщений чата.
func (s *MessageService) ClearHistory(ctx context.Context, userID, chatID string) error {
	ok, err := s.chats.IsParticipant(ctx, chatID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	return s.repo.DeleteAll(ctx, chatID)
}

// ListMedia возвращает медиа-сообщения чата.
func (s *MessageService) ListMedia(ctx context.Context, userID, chatID string, limit int) ([]domain.PublicMessage, error) {
	ok, err := s.chats.IsParticipant(ctx, chatID, userID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrForbidden
	}
	msgs, err := s.repo.ListMedia(ctx, chatID, limit)
	if err != nil {
		return nil, err
	}
	return s.toPublicBulk(ctx, msgs)
}

// DeleteForAll удаляет сообщение для всех (только admin/owner чата).
func (s *MessageService) DeleteForAll(ctx context.Context, chatID, messageID, requester string) error {
	admin, err := s.chats.IsAdmin(ctx, chatID, requester)
	if err != nil {
		return err
	}
	if !admin {
		return ErrForbidden
	}
	msg, err := s.repo.Get(ctx, messageID)
	if err != nil {
		return err
	}
	now := time.Now()
	msg.DeletedAt = &now
	return s.repo.Update(ctx, msg)
}

// React добавляет/обновляет реакцию пользователя на сообщение.
func (s *MessageService) React(ctx context.Context, messageID, userID, emoji string) error {
	msg, err := s.repo.Get(ctx, messageID)
	if err != nil {
		return err
	}
	ok, err := s.chats.IsParticipant(ctx, msg.ChatID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	return s.repo.UpsertReaction(ctx, &domain.Reaction{MessageID: messageID, UserID: userID, Emoji: emoji})
}

// RemoveReaction удаляет реакцию пользователя.
func (s *MessageService) RemoveReaction(ctx context.Context, messageID, userID, emoji string) error {
	return s.repo.DeleteReaction(ctx, messageID, userID, emoji)
}

// ListReactions возвращает агрегированные реакции по сообщению.
func (s *MessageService) ListReactions(ctx context.Context, messageID string) ([]domain.ReactionSummary, error) {
	return s.repo.ReactionsSummary(ctx, messageID)
}

// Pin закрепляет сообщение в чате (участник чата).
func (s *MessageService) Pin(ctx context.Context, chatID, messageID, userID string) error {
	ok, err := s.chats.IsParticipant(ctx, chatID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	return s.repo.Pin(ctx, chatID, messageID, userID)
}

// Unpin снимает закрепление сообщения.
func (s *MessageService) Unpin(ctx context.Context, chatID, messageID, userID string) error {
	ok, err := s.chats.IsParticipant(ctx, chatID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	return s.repo.Unpin(ctx, chatID, messageID)
}

// Forward создаёт копию сообщения в другом чате (доступно участнику целевого чата).
func (s *MessageService) Forward(ctx context.Context, messageID, userID, targetChatID string) (*domain.PublicMessage, error) {
	src, err := s.repo.Get(ctx, messageID)
	if err != nil {
		return nil, err
	}
	ok, err := s.chats.IsParticipant(ctx, targetChatID, userID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrForbidden
	}
	copy := &domain.Message{
		ChatID:   targetChatID,
		SenderID: userID,
		Type:     src.Type,
		Text:     src.Text,
		MediaURL: src.MediaURL,
	}
	if err := s.repo.Create(ctx, copy); err != nil {
		return nil, err
	}
	if err := s.chats.Touch(ctx, targetChatID); err != nil {
		log.Warn().Err(err).Msg("could not touch chat")
	}
	return s.toPublic(ctx, copy)
}

// SearchInChat ищет текстовые сообщения в рамках чата.
func (s *MessageService) SearchInChat(ctx context.Context, chatID, userID, query string, limit int) ([]domain.PublicMessage, error) {
	ok, err := s.chats.IsParticipant(ctx, chatID, userID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrForbidden
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	msgs, err := s.repo.SearchInChat(ctx, chatID, query, limit)
	if err != nil {
		return nil, err
	}
	return s.toPublicBulk(ctx, msgs)
}

// SearchGlobal ищет сообщения во всех чатах пользователя.
func (s *MessageService) SearchGlobal(ctx context.Context, userID, query string, limit int) ([]domain.PublicMessage, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	msgs, err := s.repo.SearchGlobal(ctx, userID, query, limit)
	if err != nil {
		return nil, err
	}
	return s.toPublicBulk(ctx, msgs)
}

// Typing уведомляет участников чата о том, что пользователь печатает.
func (s *MessageService) Typing(ctx context.Context, chatID, userID string) error {
	participants, err := s.chats.Participants(ctx, chatID)
	if err != nil {
		return err
	}
	recipients := participants[:0]
	for _, p := range participants {
		if p != userID {
			recipients = append(recipients, p)
		}
	}
	if s.hub != nil {
		s.hub.SendToUsers(recipients, ws.Outbound{
			Type: "message.typing",
			Payload: map[string]interface{}{
				"chat_id": chatID,
				"user_id": userID,
				"typing":  true,
			},
		})
	}
	return nil
}

// --- Проекция в публичный вид ---

func (s *MessageService) toPublic(ctx context.Context, m *domain.Message) (*domain.PublicMessage, error) {
	pm := &domain.PublicMessage{
		ID:        m.ID,
		ChatID:    m.ChatID,
		SenderID:  m.SenderID,
		Type:      m.Type,
		Text:      m.Text,
		MediaURL:  m.MediaURL,
		ReplyToID: m.ReplyToID,
		CreatedAt: m.CreatedAt.Format(time.RFC3339),
	}
	if m.EditedAt != nil {
		e := m.EditedAt.Format(time.RFC3339)
		pm.EditedAt = &e
	}

	if m.ReplyToID != nil && *m.ReplyToID != "" {
		if preview, err := s.repo.ReplyPreview(ctx, *m.ReplyToID); err == nil && preview != "" {
			pm.ReplyPreview = &preview
		}
	}
	if reactions, err := s.repo.ReactionsSummary(ctx, m.ID); err == nil {
		pm.Reactions = reactions
	}
	if pinned, err := s.repo.IsPinned(ctx, m.ID); err == nil {
		pm.Pinned = pinned
	}
	return pm, nil
}

// toPublicBulk строит публичные проекции для списка сообщений за 3 пакетных
// запроса вместо 3*N отдельных (устраняет N+1 при выдаче истории чата).
func (s *MessageService) toPublicBulk(ctx context.Context, msgs []domain.Message) ([]domain.PublicMessage, error) {
	ids := make([]string, 0, len(msgs))
	replyIDs := make([]string, 0, len(msgs))
	for i := range msgs {
		ids = append(ids, msgs[i].ID)
		if msgs[i].ReplyToID != nil && *msgs[i].ReplyToID != "" {
			replyIDs = append(replyIDs, *msgs[i].ReplyToID)
		}
	}

	reactions, err := s.repo.ReactionsSummaryBulk(ctx, ids)
	if err != nil {
		return nil, err
	}
	pinned, err := s.repo.IsPinnedBulk(ctx, ids)
	if err != nil {
		return nil, err
	}
	replyMap := map[string]string{}
	if len(replyIDs) > 0 {
		replied, err := s.repo.MessagesByIDs(ctx, replyIDs)
		if err != nil {
			return nil, err
		}
		for id, m := range replied {
			if m.Type == domain.TypeText {
				replyMap[id] = m.Text
			} else {
				replyMap[id] = string(m.Type)
			}
		}
	}

	out := make([]domain.PublicMessage, 0, len(msgs))
	for i := range msgs {
		m := &msgs[i]
		pm := domain.PublicMessage{
			ID:        m.ID,
			ChatID:    m.ChatID,
			SenderID:  m.SenderID,
			Type:      m.Type,
			Text:      m.Text,
			MediaURL:  m.MediaURL,
			ReplyToID: m.ReplyToID,
			CreatedAt: m.CreatedAt.Format(time.RFC3339),
		}
		if m.EditedAt != nil {
			e := m.EditedAt.Format(time.RFC3339)
			pm.EditedAt = &e
		}
		if m.ReplyToID != nil && *m.ReplyToID != "" {
			if preview, ok := replyMap[*m.ReplyToID]; ok {
				pm.ReplyPreview = &preview
			}
		}
		if r, ok := reactions[m.ID]; ok {
			pm.Reactions = r
		}
		pm.Pinned = pinned[m.ID]
		out = append(out, pm)
	}
	return out, nil
}
