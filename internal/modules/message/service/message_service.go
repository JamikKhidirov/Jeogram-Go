package service

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/jeogram/messenger/internal/config"
	chatrepo "github.com/jeogram/messenger/internal/modules/chat/repository"
	"github.com/jeogram/messenger/internal/modules/message/domain"
	msgrepo "github.com/jeogram/messenger/internal/modules/message/repository"
	"github.com/jeogram/messenger/internal/pkg/cache"
	"github.com/jeogram/messenger/internal/pkg/events"
	"github.com/jeogram/messenger/internal/pkg/realtime"
	"github.com/jeogram/messenger/internal/pkg/webhook"
	"github.com/jeogram/messenger/internal/pkg/ws"
	"github.com/rs/zerolog/log"
)

// ErrForbidden indicates the user may not act on the message/chat.
var ErrForbidden = errors.New("forbidden")

// MessageService implements messaging use cases and publishes domain events.
type MessageService struct {
	repo     *msgrepo.MessageRepository
	chats    *chatrepo.ChatRepository
	kafka    *events.Producer
	topics   config.KafkaConfig
	hub      realtime.Broadcaster
	msgCfg   config.MessageConfig
	redis    *cache.Redis
	webhooks *webhook.Dispatcher
}

func NewMessageService(repo *msgrepo.MessageRepository, chats *chatrepo.ChatRepository, kafka *events.Producer, topics config.KafkaConfig, hub realtime.Broadcaster, msgCfg config.MessageConfig, redis *cache.Redis, webhooks *webhook.Dispatcher) *MessageService {
	return &MessageService{repo: repo, chats: chats, kafka: kafka, topics: topics, hub: hub, msgCfg: msgCfg, redis: redis, webhooks: webhooks}
}

// msgListKey returns the Redis key for a chat's message list page.
func msgListKey(chatID string, limit, offset int) string {
	return "msgs:chat:" + chatID + ":" + strconv.Itoa(limit) + ":" + strconv.Itoa(offset)
}

// invalidateMsgCache drops cached message lists for a chat.
func (s *MessageService) invalidateMsgCache(ctx context.Context, chatID string) {
	if s.redis == nil {
		return
	}
	// Удаляем все страницы кэша сообщений чата (паттерн через DEL по ключам не
	// поддерживается в базовом клиенте, поэтому инвалидируем типовые лимиты).
	for _, lim := range []int{20, 50, 100} {
		_ = s.redis.Del(ctx, msgListKey(chatID, lim, 0))
	}
}

// Send stores a message and publishes a MessageCreated event to Kafka.
// Если указан scheduled_at в будущем, сообщение сохраняется со статусом
// "scheduled" и публикуется фоновым воркером в назначенное время.
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
		Status:   domain.StatusSent,
	}
	if req.ReplyTo != "" {
		msg.ReplyToID = &req.ReplyTo
	}

	// Отложенная публикация.
	if req.ScheduledAt != "" {
		if t, perr := time.Parse(time.RFC3339, req.ScheduledAt); perr == nil && t.After(time.Now()) {
			msg.Status = domain.StatusScheduled
			msg.ScheduledAt = &t
			msg.CreatedAt = t
		}
	}

	if err := s.repo.Create(ctx, msg); err != nil {
		return nil, err
	}
	s.invalidateMsgCache(ctx, req.ChatID)
	if err := s.chats.Touch(ctx, req.ChatID); err != nil {
		log.Warn().Err(err).Msg("could not touch chat")
	}

	// Отложенные сообщения не доставляются сразу — их опубликует воркер.
	if msg.Status == domain.StatusScheduled {
		return s.toPublic(ctx, msg)
	}

	s.deliver(ctx, msg)
	return s.toPublic(ctx, msg)
}

// deliver публикует уже сохранённое (и готовое к отправке) сообщение:
// Kafka-событие, realtime-доставка участникам и webhook-уведомление.
func (s *MessageService) deliver(ctx context.Context, msg *domain.Message) {
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
	if s.hub != nil {
		if pm, err := s.toPublic(ctx, msg); err == nil {
			if participants, perr := s.chats.Participants(ctx, msg.ChatID); perr == nil {
				s.hub.SendToUsers(participants, ws.Outbound{Type: "message.new", Payload: pm})
			}
		}
	}
	if s.webhooks != nil {
		s.webhooks.Send("message.created", event)
	}
}

// PublishDueScheduled публикует все отложенные сообщения, время которых наступило.
// Вызывается периодически из фонового воркера.
func (s *MessageService) PublishDueScheduled(ctx context.Context) (int, error) {
	due, err := s.repo.DueScheduled(ctx, time.Now())
	if err != nil {
		return 0, err
	}
	count := 0
	for _, msg := range due {
		msg.Status = domain.StatusSent
		msg.ScheduledAt = nil
		if err := s.repo.Update(ctx, &msg); err != nil {
			log.Error().Err(err).Msg("scheduled message publish failed")
			continue
		}
		s.invalidateMsgCache(ctx, msg.ChatID)
		if err := s.chats.Touch(ctx, msg.ChatID); err != nil {
			log.Warn().Err(err).Msg("could not touch chat for scheduled message")
		}
		s.deliver(ctx, &msg)
		count++
	}
	return count, nil
}

// List returns messages for a chat (access-checked), с реакциями/закрепами/ответами.
// Результат кэшируется в Redis по ключу страницы.
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
	if s.redis != nil {
		if raw, ok2, _ := s.redis.Get(ctx, msgListKey(chatID, limit, offset)); ok2 {
			var cached []domain.PublicMessage
			if err := json.Unmarshal([]byte(raw), &cached); err == nil {
				return cached, nil
			}
		}
	}
	msgs, err := s.repo.List(ctx, chatID, limit, offset)
	if err != nil {
		return nil, err
	}
	out, err := s.toPublicBulk(ctx, msgs)
	if err != nil {
		return nil, err
	}
	if s.redis != nil {
		if data, err := json.Marshal(out); err == nil {
			_ = s.redis.Set(ctx, msgListKey(chatID, limit, offset), string(data), 30*time.Second)
		}
	}
	return out, nil
}

// Edit updates the text of a message (sender only). Editing is allowed only
// within the configured window after the message was created.
func (s *MessageService) Edit(ctx context.Context, messageID, userID, text string) (*domain.PublicMessage, error) {
	msg, err := s.repo.Get(ctx, messageID)
	if err != nil {
		return nil, err
	}
	if msg.SenderID != userID {
		return nil, ErrForbidden
	}
	if s.msgCfg.EditWindow > 0 && time.Since(msg.CreatedAt) > s.msgCfg.EditWindow {
		return nil, errors.New("edit window expired")
	}
	now := time.Now()
	msg.Text = text
	msg.EditedAt = &now
	msg.EditVersion++
	if err := s.repo.Update(ctx, msg); err != nil {
		return nil, err
	}
	s.invalidateMsgCache(ctx, msg.ChatID)
	if s.hub != nil {
		if pm, err := s.toPublic(ctx, msg); err == nil {
			if participants, perr := s.chats.Participants(ctx, msg.ChatID); perr == nil {
				s.hub.SendToUsers(participants, ws.Outbound{Type: "message.edited", Payload: pm})
			}
		}
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
	if err := s.repo.Update(ctx, msg); err != nil {
		return err
	}
	s.invalidateMsgCache(ctx, msg.ChatID)
	return nil
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
	if err := s.repo.DeleteAll(ctx, chatID); err != nil {
		return err
	}
	s.invalidateMsgCache(ctx, chatID)
	return nil
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

// DeleteForAll удаляет сообщение для всех участников (только admin/owner чата).
// Удаление для всех возможно только в пределах окна после создания сообщения.
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
	if s.msgCfg.DeleteForAllWindow > 0 && time.Since(msg.CreatedAt) > s.msgCfg.DeleteForAllWindow {
		return errors.New("delete-for-all window expired")
	}
	now := time.Now()
	msg.DeletedForAllAt = &now
	if err := s.repo.Update(ctx, msg); err != nil {
		return err
	}
	s.invalidateMsgCache(ctx, msg.ChatID)
	if s.hub != nil {
		if participants, perr := s.chats.Participants(ctx, chatID); perr == nil {
			s.hub.SendToUsers(participants, ws.Outbound{Type: "message.deleted_for_all", Payload: map[string]interface{}{
				"message_id": messageID,
				"chat_id":    chatID,
			}})
		}
	}
	return nil
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
	s.invalidateMsgCache(ctx, targetChatID)
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
		Status:    m.Status,
		CreatedAt: m.CreatedAt.Format(time.RFC3339),
	}
	if m.ScheduledAt != nil {
		st := m.ScheduledAt.Format(time.RFC3339)
		pm.ScheduledAt = &st
	}
	if m.EditedAt != nil {
		e := m.EditedAt.Format(time.RFC3339)
		pm.EditedAt = &e
	}
	pm.EditVersion = m.EditVersion
	if m.DeletedForAllAt != nil {
		d := m.DeletedForAllAt.Format(time.RFC3339)
		pm.DeletedForAllAt = &d
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
			Status:    m.Status,
			CreatedAt: m.CreatedAt.Format(time.RFC3339),
		}
		if m.ScheduledAt != nil {
			st := m.ScheduledAt.Format(time.RFC3339)
			pm.ScheduledAt = &st
		}
		if m.EditedAt != nil {
			e := m.EditedAt.Format(time.RFC3339)
			pm.EditedAt = &e
		}
		pm.EditVersion = m.EditVersion
		if m.DeletedForAllAt != nil {
			d := m.DeletedForAllAt.Format(time.RFC3339)
			pm.DeletedForAllAt = &d
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
