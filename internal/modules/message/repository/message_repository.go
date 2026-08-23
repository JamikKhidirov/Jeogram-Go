package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jeogram/messenger/internal/modules/message/domain"
	"gorm.io/gorm"
)

// ErrMessageNotFound indicates a missing message.
var ErrMessageNotFound = errors.New("message not found")

// MessageRepository persists messages.
type MessageRepository struct {
	db *gorm.DB
}

func NewMessageRepository(db *gorm.DB) *MessageRepository {
	return &MessageRepository{db: db}
}

func (r *MessageRepository) Create(ctx context.Context, m *domain.Message) error {
	m.ID = uuid.NewString()
	m.CreatedAt = time.Now()
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *MessageRepository) Get(ctx context.Context, id string) (*domain.Message, error) {
	var m domain.Message
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMessageNotFound
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *MessageRepository) List(ctx context.Context, chatID string, limit, offset int) ([]domain.Message, error) {
	var msgs []domain.Message
	err := r.db.WithContext(ctx).
		Where("chat_id = ? AND deleted_at IS NULL", chatID).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&msgs).Error
	return msgs, err
}

func (r *MessageRepository) Update(ctx context.Context, m *domain.Message) error {
	return r.db.WithContext(ctx).Save(m).Error
}

func (r *MessageRepository) CountUnread(ctx context.Context, chatID, userID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Raw(`SELECT COUNT(*) FROM messages m
			 WHERE m.chat_id = ? AND m.sender_id <> ? AND m.deleted_at IS NULL
			 AND NOT EXISTS (
				SELECT 1 FROM message_reads mr
				WHERE mr.message_id = m.id AND mr.user_id = ?)`,
			chatID, userID, userID).
		Scan(&count).Error
	return count, err
}

// ReadReceipt records a per-user read receipt for a message.
func (r *MessageRepository) SaveReadReceipts(ctx context.Context, userID string, ids []string) error {
	now := time.Now()
	rows := make([]map[string]interface{}, 0, len(ids))
	for _, id := range ids {
		rows = append(rows, map[string]interface{}{
			"message_id": id,
			"user_id":    userID,
			"read_at":    now,
		})
	}
	if len(rows) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Table("message_reads").CreateInBatches(rows, 100).Error
}

// --- Реакции ---

func (r *MessageRepository) UpsertReaction(ctx context.Context, react *domain.Reaction) error {
	react.CreatedAt = time.Now()
	var existing domain.Reaction
	err := r.db.WithContext(ctx).Where("message_id = ? AND user_id = ? AND emoji = ?", react.MessageID, react.UserID, react.Emoji).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return r.db.WithContext(ctx).Create(react).Error
	}
	if err != nil {
		return err
	}
	return nil
}

func (r *MessageRepository) DeleteReaction(ctx context.Context, messageID, userID, emoji string) error {
	return r.db.WithContext(ctx).
		Where("message_id = ? AND user_id = ? AND emoji = ?", messageID, userID, emoji).
		Delete(&domain.Reaction{}).Error
}

// ReactionsSummary возвращает агрегированные реакции по эмодзи.
func (r *MessageRepository) ReactionsSummary(ctx context.Context, messageID string) ([]domain.ReactionSummary, error) {
	var reactions []domain.Reaction
	if err := r.db.WithContext(ctx).Where("message_id = ?", messageID).Find(&reactions).Error; err != nil {
		return nil, err
	}
	byEmoji := map[string]*domain.ReactionSummary{}
	for _, react := range reactions {
		s, ok := byEmoji[react.Emoji]
		if !ok {
			s = &domain.ReactionSummary{Emoji: react.Emoji}
			byEmoji[react.Emoji] = s
		}
		s.Count++
		s.Users = append(s.Users, react.UserID)
	}
	out := make([]domain.ReactionSummary, 0, len(byEmoji))
	for _, s := range byEmoji {
		out = append(out, *s)
	}
	return out, nil
}

// --- Закреплённые сообщения ---

func (r *MessageRepository) Pin(ctx context.Context, chatID, messageID, userID string) error {
	pm := domain.PinnedMessage{ChatID: chatID, MessageID: messageID, PinnedBy: userID, CreatedAt: time.Now()}
	return r.db.WithContext(ctx).Save(&pm).Error
}

func (r *MessageRepository) Unpin(ctx context.Context, chatID, messageID string) error {
	return r.db.WithContext(ctx).
		Where("chat_id = ? AND message_id = ?", chatID, messageID).
		Delete(&domain.PinnedMessage{}).Error
}

func (r *MessageRepository) IsPinned(ctx context.Context, messageID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.PinnedMessage{}).Where("message_id = ?", messageID).Count(&count).Error
	return count > 0, err
}

// --- Поиск сообщений ---

func (r *MessageRepository) SearchInChat(ctx context.Context, chatID, query string, limit int) ([]domain.Message, error) {
	var msgs []domain.Message
	like := "%" + query + "%"
	err := r.db.WithContext(ctx).
		Where("chat_id = ? AND deleted_at IS NULL AND text LIKE ?", chatID, like).
		Order("created_at DESC").Limit(limit).Find(&msgs).Error
	return msgs, err
}

func (r *MessageRepository) SearchGlobal(ctx context.Context, userID, query string, limit int) ([]domain.Message, error) {
	var msgs []domain.Message
	like := "%" + query + "%"
	err := r.db.WithContext(ctx).
		Joins("JOIN chat_participants cp ON cp.chat_id = messages.chat_id").
		Where("cp.user_id = ? AND messages.deleted_at IS NULL AND messages.text LIKE ?", userID, like).
		Order("messages.created_at DESC").Limit(limit).Find(&msgs).Error
	return msgs, err
}

// ListPinned возвращает закреплённые сообщения чата.
func (r *MessageRepository) ListPinned(ctx context.Context, chatID string, limit int) ([]domain.Message, error) {
	var pins []domain.PinnedMessage
	if err := r.db.WithContext(ctx).
		Where("chat_id = ?", chatID).
		Order("created_at DESC").Limit(limit).
		Find(&pins).Error; err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(pins))
	for _, p := range pins {
		ids = append(ids, p.MessageID)
	}
	if len(ids) == 0 {
		return []domain.Message{}, nil
	}
	byID, err := r.MessagesByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Message, 0, len(ids))
	for _, id := range ids {
		if m, ok := byID[id]; ok {
			out = append(out, m)
		}
	}
	return out, nil
}

// DeleteAll мягко удаляет все сообщения чата (очистка истории).
func (r *MessageRepository) DeleteAll(ctx context.Context, chatID string) error {
	return r.db.WithContext(ctx).
		Where("chat_id = ? AND deleted_at IS NULL", chatID).
		Delete(&domain.Message{}).Error
}

// ListMedia возвращает медиа-сообщения чата (изображения и голосовые).
func (r *MessageRepository) ListMedia(ctx context.Context, chatID string, limit int) ([]domain.Message, error) {
	var msgs []domain.Message
	err := r.db.WithContext(ctx).
		Where("chat_id = ? AND deleted_at IS NULL AND type IN ?", chatID, []string{string(domain.TypeImage), string(domain.TypeVoice)}).
		Order("created_at DESC").Limit(limit).Find(&msgs).Error
	return msgs, err
}

// ReplyPreview возвращает краткий текст сообщения, на которое идёт ответ.
func (r *MessageRepository) ReplyPreview(ctx context.Context, replyToID string) (string, error) {
	var m domain.Message
	err := r.db.WithContext(ctx).Where("id = ?", replyToID).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if m.Type == domain.TypeText {
		return m.Text, nil
	}
	return string(m.Type), nil
}

// --- Оптимизированные пакетные выборки (устраняют N+1 при списках) ---

// ReactionsSummaryBulk агрегирует реакции сразу для набора сообщений.
func (r *MessageRepository) ReactionsSummaryBulk(ctx context.Context, messageIDs []string) (map[string][]domain.ReactionSummary, error) {
	out := make(map[string][]domain.ReactionSummary, len(messageIDs))
	if len(messageIDs) == 0 {
		return out, nil
	}
	var reactions []domain.Reaction
	if err := r.db.WithContext(ctx).Where("message_id IN ?", messageIDs).Find(&reactions).Error; err != nil {
		return nil, err
	}
	groups := map[string]*domain.ReactionSummary{}
	keys := []string{}
	for _, react := range reactions {
		key := react.MessageID + "|" + react.Emoji
		s, ok := groups[key]
		if !ok {
			s = &domain.ReactionSummary{Emoji: react.Emoji}
			groups[key] = s
			keys = append(keys, key)
		}
		s.Count++
		s.Users = append(s.Users, react.UserID)
	}
	for _, key := range keys {
		idx := strings.IndexByte(key, '|')
		msgID := key[:idx]
		out[msgID] = append(out[msgID], *groups[key])
	}
	return out, nil
}

// IsPinnedBulk возвращает признак закрепления для набора сообщений.
func (r *MessageRepository) IsPinnedBulk(ctx context.Context, messageIDs []string) (map[string]bool, error) {
	out := make(map[string]bool, len(messageIDs))
	if len(messageIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		MessageID string
	}
	if err := r.db.WithContext(ctx).Model(&domain.PinnedMessage{}).
		Where("message_id IN ?", messageIDs).
		Select("message_id").Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.MessageID] = true
	}
	return out, nil
}

// MessagesByIDs загружает сообщения по ID одним запросом (для превью ответов).
func (r *MessageRepository) MessagesByIDs(ctx context.Context, ids []string) (map[string]domain.Message, error) {
	out := make(map[string]domain.Message, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var msgs []domain.Message
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&msgs).Error; err != nil {
		return nil, err
	}
	for _, m := range msgs {
		out[m.ID] = m
	}
	return out, nil
}
