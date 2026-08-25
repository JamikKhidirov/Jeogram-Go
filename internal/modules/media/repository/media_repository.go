package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jeogram/messenger/internal/modules/media/domain"
	"gorm.io/gorm"
)

// ErrMediaNotFound indicates a missing media record.
var ErrMediaNotFound = errors.New("media not found")

// MediaRepository persists media metadata keyed by UUID.
type MediaRepository struct {
	db *gorm.DB
}

// NewMediaRepository builds a MediaRepository.
func NewMediaRepository(db *gorm.DB) *MediaRepository {
	return &MediaRepository{db: db}
}

// Create stores a new media record (generating its UUID).
func (r *MediaRepository) Create(ctx context.Context, m *domain.MediaRecord) error {
	m.ID = uuid.NewString()
	return r.db.WithContext(ctx).Create(m).Error
}

// Get returns a media record by id.
func (r *MediaRepository) Get(ctx context.Context, id string) (*domain.MediaRecord, error) {
	var m domain.MediaRecord
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMediaNotFound
		}
		return nil, err
	}
	return &m, nil
}

// Delete removes a media record by id.
func (r *MediaRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&domain.MediaRecord{}).Error
}
