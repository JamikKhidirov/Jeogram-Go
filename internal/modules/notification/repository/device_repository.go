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
	return r.db.WithContext(ctx).Save(&existing).Error
}

func (r *DeviceRepository) TokensForUser(ctx context.Context, userID string) ([]domain.DeviceToken, error) {
	var tokens []domain.DeviceToken
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Find(&tokens).Error
	return tokens, err
}
