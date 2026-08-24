package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jeogram/messenger/internal/modules/auth/domain"
	"gorm.io/gorm"
)

// ErrCodeNotFound возвращается, когда нет подходящего активного кода.
var ErrCodeNotFound = errors.New("verification code not found or expired")

// VerificationRepository управляет одноразовыми кодами подтверждения.
type VerificationRepository struct {
	db *gorm.DB
}

func NewVerificationRepository(db *gorm.DB) *VerificationRepository {
	return &VerificationRepository{db: db}
}

// Create сохраняет новый код (предварительно удаляя старые по target+purpose).
func (r *VerificationRepository) Create(ctx context.Context, vc *domain.VerificationCode) error {
	vc.ID = uuid.NewString()
	vc.Used = false
	vc.CreatedAt = time.Now()
	if err := r.db.WithContext(ctx).
		Where("target = ? AND purpose = ?", vc.Target, vc.Purpose).
		Delete(&domain.VerificationCode{}).Error; err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(vc).Error
}

// FindValid ищет действующий (не использованный, не просроченный) код.
func (r *VerificationRepository) FindValid(ctx context.Context, target string, purpose domain.VerificationPurpose, code string) (*domain.VerificationCode, error) {
	var vc domain.VerificationCode
	err := r.db.WithContext(ctx).
		Where("target = ? AND purpose = ? AND code = ? AND used = ? AND expires_at > ?",
			target, purpose, code, false, time.Now()).
		First(&vc).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCodeNotFound
		}
		return nil, err
	}
	return &vc, nil
}

// MarkUsed помечает код использованным.
func (r *VerificationRepository) MarkUsed(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Model(&domain.VerificationCode{}).
		Where("id = ?", id).Update("used", true).Error
}
