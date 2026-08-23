package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jeogram/messenger/internal/modules/calls/domain"
	"gorm.io/gorm"
)

// ErrCallNotFound indicates a missing call.
var ErrCallNotFound = errors.New("call not found")

// CallRepository persists call records.
type CallRepository struct {
	db *gorm.DB
}

func NewCallRepository(db *gorm.DB) *CallRepository {
	return &CallRepository{db: db}
}

func (r *CallRepository) Create(ctx context.Context, c *domain.Call) error {
	c.ID = uuid.NewString()
	c.StartedAt = time.Now()
	c.Status = domain.CallStatusActive
	return r.db.WithContext(ctx).Create(c).Error
}

func (r *CallRepository) End(ctx context.Context, id string) error {
	var c domain.Call
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&c).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCallNotFound
		}
		return err
	}
	now := time.Now()
	c.Status = domain.CallStatusEnded
	c.EndedAt = &now
	return r.db.WithContext(ctx).Save(&c).Error
}

// Get returns a single call record.
func (r *CallRepository) Get(ctx context.Context, id string) (*domain.Call, error) {
	var c domain.Call
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&c).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCallNotFound
		}
		return nil, err
	}
	return &c, nil
}
