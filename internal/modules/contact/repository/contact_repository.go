package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jeogram/messenger/internal/modules/contact/domain"
	"gorm.io/gorm"
)

// ErrContactNotFound возвращается, когда связь не найдена.
var ErrContactNotFound = errors.New("contact not found")

// ContactRepository управляет связями контактов.
type ContactRepository struct {
	db *gorm.DB
}

func NewContactRepository(db *gorm.DB) *ContactRepository {
	return &ContactRepository{db: db}
}

// Create создаёт запрос в контакты (owner -> contact). Если связь уже есть — возвращает её.
func (r *ContactRepository) Create(ctx context.Context, owner, contact string) (*domain.Contact, error) {
	var existing domain.Contact
	err := r.db.WithContext(ctx).Where("owner_id = ? AND contact_id = ?", owner, contact).First(&existing).Error
	if err == nil {
		return &existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	c := domain.Contact{
		ID:        uuid.NewString(),
		OwnerID:   owner,
		ContactID: contact,
		Status:    domain.ContactPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := r.db.WithContext(ctx).Create(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

// Get возвращает конкретную связь owner -> contact.
func (r *ContactRepository) Get(ctx context.Context, owner, contact string) (*domain.Contact, error) {
	var c domain.Contact
	err := r.db.WithContext(ctx).Where("owner_id = ? AND contact_id = ?", owner, contact).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrContactNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// Accept подтверждает входящий запрос (owner -> contact становится accepted).
func (r *ContactRepository) Accept(ctx context.Context, owner, contact string) error {
	res := r.db.WithContext(ctx).Model(&domain.Contact{}).
		Where("owner_id = ? AND contact_id = ?", owner, contact).
		Update("status", domain.ContactAccepted)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrContactNotFound
	}
	return nil
}

// Delete удаляет связь в любом направлении между двумя пользователями.
func (r *ContactRepository) Delete(ctx context.Context, userA, userB string) error {
	return r.db.WithContext(ctx).
		Where("(owner_id = ? AND contact_id = ?) OR (owner_id = ? AND contact_id = ?)", userA, userB, userB, userA).
		Delete(&domain.Contact{}).Error
}

// ListAccepted возвращает подтверждённые контакты пользователя.
func (r *ContactRepository) ListAccepted(ctx context.Context, owner string) ([]domain.Contact, error) {
	var list []domain.Contact
	err := r.db.WithContext(ctx).Where("owner_id = ? AND status = ?", owner, domain.ContactAccepted).Find(&list).Error
	return list, err
}

// ListIncomingRequests возвращает входящие (pending) запросы, адресованные пользователю.
func (r *ContactRepository) ListIncomingRequests(ctx context.Context, contact string) ([]domain.Contact, error) {
	var list []domain.Contact
	err := r.db.WithContext(ctx).Where("contact_id = ? AND status = ?", contact, domain.ContactPending).Find(&list).Error
	return list, err
}

// DeleteAllForUser удаляет все связи контактов пользователя (при удалении аккаунта).
func (r *ContactRepository) DeleteAllForUser(ctx context.Context, userID string) error {
	return r.db.WithContext(ctx).
		Where("owner_id = ? OR contact_id = ?", userID, userID).
		Delete(&domain.Contact{}).Error
}

// ListIDs возвращает id всех подтверждённых контактов владельца.
func (r *ContactRepository) ListIDs(ctx context.Context, owner string) ([]string, error) {
	var ids []string
	err := r.db.WithContext(ctx).
		Model(&domain.Contact{}).
		Where("owner_id = ? AND status = ?", owner, domain.ContactAccepted).
		Pluck("contact_id", &ids).Error
	return ids, err
}
