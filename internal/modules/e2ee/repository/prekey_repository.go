package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jeogram/messenger/internal/modules/e2ee/domain"
	"gorm.io/gorm"
)

// ErrNoPreKey indicates no unused prekey is available for the user.
var ErrNoPreKey = errors.New("no unused prekey available")

// PreKeyRepository persists one-time prekeys.
type PreKeyRepository struct {
	db *gorm.DB
}

func NewPreKeyRepository(db *gorm.DB) *PreKeyRepository {
	return &PreKeyRepository{db: db}
}

// Upsert replaces a user's prekey bundle with a fresh batch.
func (r *PreKeyRepository) Upsert(ctx context.Context, userID string, items []domain.PreKeyItem) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&domain.PreKey{}).Error; err != nil {
			return err
		}
		now := time.Now()
		rows := make([]domain.PreKey, 0, len(items))
		for _, it := range items {
			rows = append(rows, domain.PreKey{
				ID:           uuid.NewString(),
				UserID:       userID,
				KeyID:        it.KeyID,
				PublicKey:    it.PublicKey,
				SignatureKey: it.SignatureKey,
				Used:         false,
				CreatedAt:    now,
			})
		}
		return tx.Create(&rows).Error
	})
}

// ClaimOne returns and marks used a single unused prekey for the user.
func (r *PreKeyRepository) ClaimOne(ctx context.Context, userID string) (*domain.PreKey, error) {
	var pk domain.PreKey
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND used = ?", userID, false).
		Order("created_at ASC").
		First(&pk).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNoPreKey
	}
	if err != nil {
		return nil, err
	}
	pk.Used = true
	if err := r.db.WithContext(ctx).Model(&domain.PreKey{}).Where("id = ?", pk.ID).Update("used", true).Error; err != nil {
		return nil, err
	}
	return &pk, nil
}
