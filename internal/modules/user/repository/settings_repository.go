package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jeogram/messenger/internal/modules/user/domain"
	"gorm.io/gorm"
)

// SettingsRepository persists user settings.
type SettingsRepository struct {
	db *gorm.DB
}

func NewSettingsRepository(db *gorm.DB) *SettingsRepository {
	return &SettingsRepository{db: db}
}

func (r *SettingsRepository) Get(ctx context.Context, userID string) (*domain.UserSettings, error) {
	var s domain.UserSettings
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&s).Error
	if err == gorm.ErrRecordNotFound {
		s = domain.UserSettings{UserID: userID, Theme: "system", Language: "en", NotificationsEnabled: true, OnlineVisible: true}
		return &s, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *SettingsRepository) Upsert(ctx context.Context, s *domain.UserSettings) error {
	var existing domain.UserSettings
	err := r.db.WithContext(ctx).Where("user_id = ?", s.UserID).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return r.db.WithContext(ctx).Create(s).Error
	}
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Save(s).Error
}

// --- Блокировки ---

func (r *SettingsRepository) Block(ctx context.Context, blocker, blocked string) error {
	bu := domain.BlockedUser{BlockerID: blocker, BlockedID: blocked, CreatedAt: time.Now()}
	var existing domain.BlockedUser
	err := r.db.WithContext(ctx).Where("blocker_id = ? AND blocked_id = ?", blocker, blocked).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return r.db.WithContext(ctx).Create(&bu).Error
	}
	return err
}

func (r *SettingsRepository) Unblock(ctx context.Context, blocker, blocked string) error {
	return r.db.WithContext(ctx).
		Where("blocker_id = ? AND blocked_id = ?", blocker, blocked).
		Delete(&domain.BlockedUser{}).Error
}

func (r *SettingsRepository) ListBlocks(ctx context.Context, blocker string) ([]domain.BlockedUser, error) {
	var list []domain.BlockedUser
	err := r.db.WithContext(ctx).Where("blocker_id = ?", blocker).Find(&list).Error
	return list, err
}

func (r *SettingsRepository) IsBlocked(ctx context.Context, blocker, blocked string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.BlockedUser{}).
		Where("blocker_id = ? AND blocked_id = ?", blocker, blocked).Count(&count).Error
	return count > 0, err
}

// DeleteAllForUser удаляет настройки и блокировки пользователя (при удалении аккаунта).
func (r *SettingsRepository) DeleteAllForUser(ctx context.Context, userID string) error {
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&domain.UserSettings{}).Error; err != nil {
		return err
	}
	return r.db.WithContext(ctx).
		Where("blocker_id = ? OR blocked_id = ?", userID, userID).
		Delete(&domain.BlockedUser{}).Error
}
