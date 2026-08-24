package repository

import (
	"context"

	"github.com/jeogram/messenger/internal/modules/notification/domain"
	"gorm.io/gorm"
)

// DeviceRepository persists push device tokens.
type DeviceRepository struct {
	db *gorm.DB
}

func NewDeviceRepository(db *gorm.DB) *DeviceRepository {
	return &DeviceRepository{db: db}
}

func (r *DeviceRepository) Upsert(ctx context.Context, d *domain.DeviceToken) error {
	var existing domain.DeviceToken
	err := r.db.WithContext(ctx).Where("user_id = ? AND platform = ?", d.UserID, d.Platform).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return r.db.WithContext(ctx).Create(d).Error
	}
	if err != nil {
		return err
	}
	existing.Token = d.Token
	existing.DeviceModel = d.DeviceModel
	existing.OSVersion = d.OSVersion
	existing.AppVersion = d.AppVersion
	existing.Locale = d.Locale
	existing.Timezone = d.Timezone
	existing.LastIP = d.LastIP
	existing.UserAgent = d.UserAgent
	existing.UpdatedAt = d.UpdatedAt
	return r.db.WithContext(ctx).Save(&existing).Error
}

func (r *DeviceRepository) TokensForUser(ctx context.Context, userID string) ([]domain.DeviceToken, error) {
	var tokens []domain.DeviceToken
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Find(&tokens).Error
	return tokens, err
}

// ListAll returns all registered devices (with telemetry) for admin views.
func (r *DeviceRepository) ListAll(ctx context.Context, limit, offset int) ([]domain.DeviceToken, error) {
	var tokens []domain.DeviceToken
	err := r.db.WithContext(ctx).
		Order("updated_at DESC").
		Limit(limit).Offset(offset).
		Find(&tokens).Error
	return tokens, err
}

// Delete removes a single device registration (remote logout of that device).
func (r *DeviceRepository) Delete(ctx context.Context, userID, platform string) error {
	return r.db.WithContext(ctx).
		Where("user_id = ? AND platform = ?", userID, platform).
		Delete(&domain.DeviceToken{}).Error
}
