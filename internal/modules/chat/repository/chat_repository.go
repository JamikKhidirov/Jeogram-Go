package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jeogram/messenger/internal/modules/chat/domain"
	"gorm.io/gorm"
)

// ErrChatNotFound indicates a missing chat.
var ErrChatNotFound = errors.New("chat not found")

// ErrNotParticipant indicates the user is not part of the chat.
var ErrNotParticipant = errors.New("user is not a participant of this chat")

// ChatRepository persists chats and participants.
type ChatRepository struct {
	db *gorm.DB
}

func NewChatRepository(db *gorm.DB) *ChatRepository {
	return &ChatRepository{db: db}
}

func (r *ChatRepository) Create(ctx context.Context, chat *domain.Chat, participants []string) error {
	chat.ID = uuid.NewString()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(chat).Error; err != nil {
			return err
		}
		for _, uid := range participants {
			cp := domain.ChatParticipant{
				ChatID:   chat.ID,
				UserID:   uid,
				Role:     "member",
				JoinedAt: chat.CreatedAt,
			}
			if err := tx.Create(&cp).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// FindPrivateChat returns an existing 1:1 chat between two users, if any.
func (r *ChatRepository) FindPrivateChat(ctx context.Context, userA, userB string) (*domain.Chat, error) {
	var chat domain.Chat
	err := r.db.WithContext(ctx).
		Raw(`SELECT c.* FROM chats c
				JOIN chat_participants p1 ON p1.chat_id = c.id AND p1.user_id = ?
				JOIN chat_participants p2 ON p2.chat_id = c.id AND p2.user_id = ?
				WHERE c.type = 'private' LIMIT 1`, userA, userB).
		Scan(&chat).Error
	if err != nil {
		return nil, err
	}
	if chat.ID == "" {
		return nil, ErrChatNotFound
	}
	return &chat, nil
}

func (r *ChatRepository) Get(ctx context.Context, chatID string) (*domain.Chat, error) {
	var chat domain.Chat
	err := r.db.WithContext(ctx).Where("id = ?", chatID).First(&chat).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrChatNotFound
	}
	if err != nil {
		return nil, err
	}
	return &chat, nil
}

func (r *ChatRepository) ListForUser(ctx context.Context, userID string) ([]domain.Chat, error) {
	var chats []domain.Chat
	err := r.db.WithContext(ctx).
		Joins("JOIN chat_participants cp ON cp.chat_id = chats.id").
		Where("cp.user_id = ?", userID).
		Order("chats.updated_at DESC").
		Find(&chats).Error
	return chats, err
}

// Search возвращает чаты пользователя, название которых содержит query.
func (r *ChatRepository) Search(ctx context.Context, userID, query string, limit int) ([]domain.Chat, error) {
	var chats []domain.Chat
	like := "%" + query + "%"
	err := r.db.WithContext(ctx).
		Joins("JOIN chat_participants cp ON cp.chat_id = chats.id").
		Where("cp.user_id = ?", userID).
		Where("LOWER(chats.title) LIKE LOWER(?)", like).
		Order("chats.updated_at DESC").
		Limit(limit).Find(&chats).Error
	return chats, err
}

func (r *ChatRepository) Participants(ctx context.Context, chatID string) ([]string, error) {
	var ids []string
	err := r.db.WithContext(ctx).
		Model(&domain.ChatParticipant{}).
		Where("chat_id = ?", chatID).
		Pluck("user_id", &ids).Error
	return ids, err
}

func (r *ChatRepository) IsParticipant(ctx context.Context, chatID, userID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&domain.ChatParticipant{}).
		Where("chat_id = ? AND user_id = ?", chatID, userID).
		Count(&count).Error
	return count > 0, err
}

// GetParticipant возвращает запись участника чата.
func (r *ChatRepository) GetParticipant(ctx context.Context, chatID, userID string) (*domain.ChatParticipant, error) {
	var cp domain.ChatParticipant
	err := r.db.WithContext(ctx).Where("chat_id = ? AND user_id = ?", chatID, userID).First(&cp).Error
	if err != nil {
		return nil, err
	}
	return &cp, nil
}

// IsAdmin reports whether the user is an admin or owner of the chat.
func (r *ChatRepository) IsAdmin(ctx context.Context, chatID, userID string) (bool, error) {
	cp, err := r.GetParticipant(ctx, chatID, userID)
	if err != nil {
		return false, err
	}
	return cp.Role == string(domain.RoleAdmin) || cp.Role == string(domain.RoleOwner), nil
}

// IsOwner reports whether the user is the owner of the chat.
func (r *ChatRepository) IsOwner(ctx context.Context, chatID, userID string) (bool, error) {
	cp, err := r.GetParticipant(ctx, chatID, userID)
	if err != nil {
		return false, err
	}
	return cp.Role == string(domain.RoleOwner), nil
}

// SetRole changes the role of a participant.
func (r *ChatRepository) SetRole(ctx context.Context, chatID, userID, role string) error {
	return r.db.WithContext(ctx).Model(&domain.ChatParticipant{}).
		Where("chat_id = ? AND user_id = ?", chatID, userID).
		Update("role", role).Error
}

// RemoveParticipant deletes a participant from the chat.
func (r *ChatRepository) RemoveParticipant(ctx context.Context, chatID, userID string) error {
	return r.db.WithContext(ctx).Where("chat_id = ? AND user_id = ?", chatID, userID).
		Delete(&domain.ChatParticipant{}).Error
}

// UpdateChat saves title/avatar changes.
func (r *ChatRepository) UpdateChat(ctx context.Context, chat *domain.Chat) error {
	return r.db.WithContext(ctx).Model(chat).Updates(map[string]interface{}{
		"title":      chat.Title,
		"avatar_url": chat.AvatarURL,
	}).Error
}

func (r *ChatRepository) AddParticipant(ctx context.Context, chatID, userID string) error {
	cp := domain.ChatParticipant{ChatID: chatID, UserID: userID, Role: "member", JoinedAt: time.Now()}
	return r.db.WithContext(ctx).Create(&cp).Error
}

// SetMuted toggles the mute flag for a participant (silences push/notifications).
func (r *ChatRepository) SetMuted(ctx context.Context, chatID, userID string, muted bool) error {
	return r.db.WithContext(ctx).Model(&domain.ChatParticipant{}).
		Where("chat_id = ? AND user_id = ?", chatID, userID).
		Update("muted", muted).Error
}

func (r *ChatRepository) Touch(ctx context.Context, chatID string) error {
	return r.db.WithContext(ctx).Model(&domain.Chat{}).Where("id = ?", chatID).
		Update("updated_at", time.Now()).Error
}

// RemoveUserEverywhere удаляет пользователя из всех чатов (при удалении аккаунта).
func (r *ChatRepository) RemoveUserEverywhere(ctx context.Context, userID string) error {
	return r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&domain.ChatParticipant{}).Error
}
