package service

import (
	"context"
	"errors"
	"time"

	"github.com/jeogram/messenger/internal/modules/user/domain"
	"github.com/jeogram/messenger/internal/modules/user/repository"
)

type UserStatusService struct {
	userRepo     *repository.UserRepository
	settingsRepo *repository.SettingsRepository
}

func NewUserStatusService(userRepo *repository.UserRepository, settingsRepo *repository.SettingsRepository) *UserStatusService {
	return &UserStatusService{userRepo: userRepo, settingsRepo: settingsRepo}
}

func (s *UserStatusService) UpdateStatus(ctx context.Context, userID string, status domain.UserStatus) error {
	if status != domain.StatusOnline && status != domain.StatusOffline && 
	   status != domain.StatusAway && status != domain.StatusBusy && status != domain.StatusDND {
		return errors.New("invalid status")
	}

	userStatus := &domain.UserStatusModel{
		ID:        userID,
		UserID:    userID,
		Status:    status,
		UpdatedAt: time.Now(),
	}

	return s.userRepo.UpdateStatus(ctx, userStatus)
}

func (s *UserStatusService) GetStatus(ctx context.Context, userID string) (domain.UserStatus, error) {
	status, err := s.userRepo.GetStatus(ctx, userID)
	if err != nil {
		return domain.StatusOffline, err
	}
	return status, nil
}

func (s *UserStatusService) UpdateSettings(ctx context.Context, userID string, req domain.UpdateSettingsRequest) (*domain.UserSettingsModel, error) {
	settings, err := s.settingsRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if req.NotificationsEnabled != nil {
		settings.NotificationsEnabled = *req.NotificationsEnabled
	}
	if req.ReadReceipts != nil {
		settings.ReadReceipts = *req.ReadReceipts
	}
	if req.TypingIndicators != nil {
		settings.TypingIndicators = *req.TypingIndicators
	}
	if req.OnlineStatus != nil {
		settings.OnlineStatus = *req.OnlineStatus
	}
	if req.Theme != nil {
		settings.Theme = *req.Theme
	}
	if req.Language != nil {
		settings.Language = *req.Language
	}
	if req.Privacy != nil {
		settings.Privacy = *req.Privacy
	}

	settings.UpdatedAt = time.Now()

	if err := s.settingsRepo.Update(ctx, settings); err != nil {
		return nil, err
	}
	return settings, nil
}

func (s *UserStatusService) GetSettings(ctx context.Context, userID string) (*domain.UserSettingsModel, error) {
	return s.settingsRepo.GetByUserID(ctx, userID)
}

func (s *UserStatusService) BlockUser(ctx context.Context, userID, blockedUserID, reason string) error {
	if userID == blockedUserID {
		return errors.New("cannot block yourself")
	}
	return s.userRepo.BlockUser(ctx, userID, blockedUserID, reason)
}

func (s *UserStatusService) UnblockUser(ctx context.Context, userID, blockedUserID string) error {
	return s.userRepo.UnblockUser(ctx, userID, blockedUserID)
}

func (s *UserStatusService) GetBlockedUsers(ctx context.Context, userID string) ([]map[string]interface{}, error) {
	return s.userRepo.GetBlockedUsers(ctx, userID)
}

func (s *UserStatusService) IsBlockedBy(ctx context.Context, userID, otherUserID string) (bool, error) {
	return s.userRepo.IsBlockedBy(ctx, userID, otherUserID)
}
