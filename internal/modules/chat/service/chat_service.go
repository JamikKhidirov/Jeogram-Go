package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jeogram/messenger/internal/modules/chat/domain"
	"github.com/jeogram/messenger/internal/modules/chat/repository"
	"github.com/jeogram/messenger/internal/pkg/cache"
)

// ChatService implements chat use cases.
type ChatService struct {
	repo  *repository.ChatRepository
	redis *cache.Redis
}

func NewChatService(repo *repository.ChatRepository, redis *cache.Redis) *ChatService {
	return &ChatService{repo: repo, redis: redis}
}

// Ошибки сервиса чатов.
var (
	ErrForbidden = errors.New("forbidden: недостаточно прав")
)

// chatListKey returns the Redis key for a user's chat list.
func chatListKey(userID string) string { return "chats:user:" + userID }

// invalidateChatList drops cached chat lists for all participants of a chat.
func (s *ChatService) invalidateChatList(ctx context.Context, chatID string) {
	if s.redis == nil {
		return
	}
	if parts, err := s.repo.Participants(ctx, chatID); err == nil {
		keys := make([]string, 0, len(parts))
		for _, p := range parts {
			keys = append(keys, chatListKey(p))
		}
		_ = s.redis.Del(ctx, keys...)
	}
}

// CreatePrivateChat returns an existing 1:1 chat or creates one.
func (s *ChatService) CreatePrivateChat(ctx context.Context, initiator, other string) (*domain.PublicChat, error) {
	if initiator == other {
		return nil, errors.New("cannot create a private chat with yourself")
	}
	chat, err := s.repo.FindPrivateChat(ctx, initiator, other)
	if err == repository.ErrChatNotFound {
		chat = &domain.Chat{Type: domain.ChatTypePrivate, CreatedAt: time.Now(), UpdatedAt: time.Now()}
		if err := s.repo.Create(ctx, chat, []string{initiator, other}); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	s.invalidateChatList(ctx, chat.ID)
	return s.toPublic(ctx, chat)
}

// CreateGroupChat creates a new group conversation including the creator.
// Создатель становится владельцем (owner) чата.
func (s *ChatService) CreateGroupChat(ctx context.Context, creator string, req domain.CreateGroupChatRequest) (*domain.PublicChat, error) {
	participants := append([]string{creator}, req.ParticipantIDs...)
	chat := &domain.Chat{
		Type:      domain.ChatTypeGroup,
		Title:     req.Title,
		AvatarURL: req.AvatarURL,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := s.repo.Create(ctx, chat, participants); err != nil {
		return nil, err
	}
	// Назначаем создателя владельцем.
	if err := s.repo.SetRole(ctx, chat.ID, creator, domain.RoleOwner); err != nil {
		return nil, err
	}
	s.invalidateChatList(ctx, chat.ID)
	return s.toPublic(ctx, chat)
}

// UpdateChat меняет название/аватар группы (только admin/owner).
func (s *ChatService) UpdateChat(ctx context.Context, chatID, requester, title, avatarURL string) (*domain.PublicChat, error) {
	admin, err := s.repo.IsAdmin(ctx, chatID, requester)
	if err != nil {
		return nil, err
	}
	if !admin {
		return nil, ErrForbidden
	}
	chat, err := s.repo.Get(ctx, chatID)
	if err != nil {
		return nil, err
	}
	if title != "" {
		chat.Title = title
	}
	if avatarURL != "" {
		chat.AvatarURL = avatarURL
	}
	if err := s.repo.UpdateChat(ctx, chat); err != nil {
		return nil, err
	}
	return s.toPublic(ctx, chat)
}

// Promote назначает пользователя администратором (только owner).
func (s *ChatService) Promote(ctx context.Context, chatID, requester, target string) error {
	owner, err := s.repo.IsOwner(ctx, chatID, requester)
	if err != nil {
		return err
	}
	if !owner {
		return ErrForbidden
	}
	return s.repo.SetRole(ctx, chatID, target, domain.RoleAdmin)
}

// Demote понижает администратора до участника (только owner).
func (s *ChatService) Demote(ctx context.Context, chatID, requester, target string) error {
	owner, err := s.repo.IsOwner(ctx, chatID, requester)
	if err != nil {
		return err
	}
	if !owner {
		return ErrForbidden
	}
	return s.repo.SetRole(ctx, chatID, target, domain.RoleMember)
}

// RemoveParticipant удаляет участника из группы (admin/owner, нельзя удалить владельца).
func (s *ChatService) RemoveParticipant(ctx context.Context, chatID, requester, target string) error {
	admin, err := s.repo.IsAdmin(ctx, chatID, requester)
	if err != nil {
		return err
	}
	if !admin {
		return ErrForbidden
	}
	isOwner, err := s.repo.IsOwner(ctx, chatID, target)
	if err != nil {
		return err
	}
	if isOwner {
		return errors.New("нельзя удалить владельца чата")
	}
	if err := s.repo.RemoveParticipant(ctx, chatID, target); err != nil {
		return err
	}
	s.invalidateChatList(ctx, chatID)
	return nil
}

// ListChats returns all chats a user participates in (кэшируется в Redis).
func (s *ChatService) ListChats(ctx context.Context, userID string) ([]domain.PublicChat, error) {
	if s.redis != nil {
		if raw, ok, _ := s.redis.Get(ctx, chatListKey(userID)); ok {
			var cached []domain.PublicChat
			if err := json.Unmarshal([]byte(raw), &cached); err == nil {
				return cached, nil
			}
		}
	}
	chats, err := s.repo.ListForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.PublicChat, 0, len(chats))
	for i := range chats {
		p, err := s.toPublic(ctx, &chats[i])
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	if s.redis != nil {
		if data, err := json.Marshal(out); err == nil {
			_ = s.redis.Set(ctx, chatListKey(userID), string(data), 60*time.Second)
		}
	}
	return out, nil
}

// SearchChats ищет чаты пользователя по названию.
func (s *ChatService) SearchChats(ctx context.Context, userID, query string, limit int) ([]domain.PublicChat, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	chats, err := s.repo.Search(ctx, userID, query, limit)
	if err != nil {
		return nil, err
	}
	out := make([]domain.PublicChat, 0, len(chats))
	for i := range chats {
		p, err := s.toPublic(ctx, &chats[i])
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, nil
}

// LeaveChat удаляет пользователя из чата (покидает чат "тихо", без уведомлений).
func (s *ChatService) LeaveChat(ctx context.Context, userID, chatID string) error {
	ok, err := s.repo.IsParticipant(ctx, chatID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return repository.ErrNotParticipant
	}
	if err := s.repo.RemoveParticipant(ctx, chatID, userID); err != nil {
		return err
	}
	s.invalidateChatList(ctx, chatID)
	return nil
}

// EnableE2EE переводит чат в режим сквозного шифрования (E2EE). Сервер
// становится "слепым" — он хранит только зашифрованный ciphertext сообщений.
// Включить может любой участник чата (обычно по взаимному согласию).
func (s *ChatService) EnableE2EE(ctx context.Context, chatID, userID string) (*domain.PublicChat, error) {
	ok, err := s.repo.IsParticipant(ctx, chatID, userID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, repository.ErrNotParticipant
	}
	if err := s.repo.SetEncryption(ctx, chatID, domain.EncE2EE); err != nil {
		return nil, err
	}
	s.invalidateChatList(ctx, chatID)
	return s.GetChat(ctx, chatID, userID)
}

// AddParticipant adds a user to a group chat.
func (s *ChatService) AddParticipant(ctx context.Context, chatID, userID, requester string) (*domain.PublicChat, error) {
	ok, err := s.repo.IsParticipant(ctx, chatID, requester)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, repository.ErrNotParticipant
	}
	if err := s.repo.AddParticipant(ctx, chatID, userID); err != nil {
		return nil, err
	}
	s.invalidateChatList(ctx, chatID)
	chat, err := s.repo.Get(ctx, chatID)
	if err != nil {
		return nil, err
	}
	return s.toPublic(ctx, chat)
}

// GetChat возвращает информацию о чате (с проверкой доступа).
func (s *ChatService) GetChat(ctx context.Context, chatID, requester string) (*domain.PublicChat, error) {
	ok, err := s.repo.IsParticipant(ctx, chatID, requester)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, repository.ErrNotParticipant
	}
	chat, err := s.repo.Get(ctx, chatID)
	if err != nil {
		return nil, err
	}
	return s.toPublic(ctx, chat)
}

// Participants returns the user ids in a chat (access-checked).
// MuteChat toggles the mute flag for the requester in the given chat.
func (s *ChatService) MuteChat(ctx context.Context, userID, chatID string, muted bool) error {
	isParticipant, err := s.repo.IsParticipant(ctx, chatID, userID)
	if err != nil {
		return err
	}
	if !isParticipant {
		return errors.New("not a participant of this chat")
	}
	return s.repo.SetMuted(ctx, chatID, userID, muted)
}

func (s *ChatService) Participants(ctx context.Context, chatID, requester string) ([]string, error) {
	ok, err := s.repo.IsParticipant(ctx, chatID, requester)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, repository.ErrNotParticipant
	}
	return s.repo.Participants(ctx, chatID)
}

func (s *ChatService) toPublic(ctx context.Context, chat *domain.Chat) (*domain.PublicChat, error) {
	parts, err := s.repo.Participants(ctx, chat.ID)
	if err != nil {
		return nil, err
	}
	return &domain.PublicChat{
		ID:           chat.ID,
		Type:         chat.Type,
		Title:        chat.Title,
		AvatarURL:    chat.AvatarURL,
		Participants: parts,
		CreatedAt:    chat.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}
