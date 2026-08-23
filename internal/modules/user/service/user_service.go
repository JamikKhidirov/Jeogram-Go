package service

import (
	"context"
	"errors"

	authdomain "github.com/jeogram/messenger/internal/modules/auth/domain"
	authrepo "github.com/jeogram/messenger/internal/modules/auth/repository"
	contactrepo "github.com/jeogram/messenger/internal/modules/contact/repository"
	chatrepo "github.com/jeogram/messenger/internal/modules/chat/repository"
	"github.com/jeogram/messenger/internal/modules/user/domain"
	userrepo "github.com/jeogram/messenger/internal/modules/user/repository"
)

// UserService manages profiles and settings.
type UserService struct {
	users    *authrepo.UserRepository
	settings *userrepo.SettingsRepository
	contacts *contactrepo.ContactRepository
	chats    *chatrepo.ChatRepository
}

func NewUserService(users *authrepo.UserRepository, settings *userrepo.SettingsRepository, contacts *contactrepo.ContactRepository, chats *chatrepo.ChatRepository) *UserService {
	return &UserService{users: users, settings: settings, contacts: contacts, chats: chats}
}

// GetProfile returns a public profile for a user.
func (s *UserService) GetProfile(ctx context.Context, userID string) (*authdomain.PublicUser, error) {
	u, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return authdomain.ToPublic(u), nil
}

// UpdateProfile edits display name / avatar / username.
func (s *UserService) UpdateProfile(ctx context.Context, userID string, req domain.UpdateProfileRequest) (*authdomain.PublicUser, error) {
	u, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if req.DisplayName != "" {
		u.DisplayName = req.DisplayName
	}
	if req.AvatarURL != "" {
		u.AvatarURL = req.AvatarURL
	}
	if req.Username != "" {
		u.Username = req.Username
	}
	if err := s.users.Update(ctx, u); err != nil {
		return nil, err
	}
	return authdomain.ToPublic(u), nil
}

// GetSettings returns the user's settings, creating defaults if missing.
func (s *UserService) GetSettings(ctx context.Context, userID string) (*domain.UserSettings, error) {
	return s.settings.Get(ctx, userID)
}

// UpdateSettings applies changes to user settings.
func (s *UserService) UpdateSettings(ctx context.Context, userID string, req domain.UpdateSettingsRequest) (*domain.UserSettings, error) {
	sets, err := s.settings.Get(ctx, userID)
	if err != nil {
		return nil, err
	}
	if req.Theme != "" {
		sets.Theme = req.Theme
	}
	if req.Language != "" {
		sets.Language = req.Language
	}
	if req.NotificationsEnabled != nil {
		sets.NotificationsEnabled = *req.NotificationsEnabled
	}
	if req.OnlineVisible != nil {
		sets.OnlineVisible = *req.OnlineVisible
	}
	if err := s.settings.Upsert(ctx, sets); err != nil {
		return nil, err
	}
	return sets, nil
}

// Search finds users by username or display name.
func (s *UserService) Search(ctx context.Context, query string, limit int) ([]authdomain.PublicUser, error) {
	users, err := s.users.Search(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	out := make([]authdomain.PublicUser, 0, len(users))
	for i := range users {
		out = append(out, *authdomain.ToPublic(&users[i]))
	}
	return out, nil
}

// Block добавляет пользователя в чёрный список.
func (s *UserService) Block(ctx context.Context, blocker, blocked string) error {
	if blocker == blocked {
		return errors.New("cannot block yourself")
	}
	return s.settings.Block(ctx, blocker, blocked)
}

// Unblock убирает пользователя из чёрного списка.
func (s *UserService) Unblock(ctx context.Context, blocker, blocked string) error {
	return s.settings.Unblock(ctx, blocker, blocked)
}

// ListBlocks возвращает чёрный список пользователя.
func (s *UserService) ListBlocks(ctx context.Context, blocker string) ([]domain.BlockedUser, error) {
	return s.settings.ListBlocks(ctx, blocker)
}

// DeleteAccount удаляет аккаунт пользователя и все связанные данные.
func (s *UserService) DeleteAccount(ctx context.Context, userID string) error {
	if s.contacts != nil {
		if err := s.contacts.DeleteAllForUser(ctx, userID); err != nil {
			return err
		}
	}
	if err := s.settings.DeleteAllForUser(ctx, userID); err != nil {
		return err
	}
	if s.chats != nil {
		if err := s.chats.RemoveUserEverywhere(ctx, userID); err != nil {
			return err
		}
	}
	return s.users.Delete(ctx, userID)
}
