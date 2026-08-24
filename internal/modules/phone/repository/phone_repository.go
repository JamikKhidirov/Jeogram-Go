package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// Storer — минимальный контракт для универсального хранилища записей телефона.
type Storer interface {
	BatchCreate(ctx context.Context, items any) error
	ListAll(ctx context.Context, userID string, limit, offset int) (any, error)
	ClearAll(ctx context.Context, userID string) error
	CountAll(ctx context.Context, userID string) (int64, error)
}

// PhoneStore — дженерик-репозиторий для любой модели телефона.
// Используется всеми категориями сбора данных (устройство, гео, приложения и т.д.).
type PhoneStore[M any] struct {
	db *gorm.DB
}

// NewPhoneStore создаёт хранилище для конкретной модели M.
func NewPhoneStore[M any](db *gorm.DB) *PhoneStore[M] {
	return &PhoneStore[M]{db: db}
}

// BatchCreate сохраняет пакет записей (items должен быть *[]M).
func (s *PhoneStore[M]) BatchCreate(ctx context.Context, items any) error {
	ptr, ok := items.(*[]M)
	if !ok {
		return fmt.Errorf("phone store: ожидался *[]M, получен %T", items)
	}
	if len(*ptr) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).Create(ptr).Error
}

// ListAll возвращает записи пользователя, отсортированные по убыванию created_at.
func (s *PhoneStore[M]) ListAll(ctx context.Context, userID string, limit, offset int) (any, error) {
	var out []M
	q := s.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at DESC")
	if limit > 0 {
		q = q.Limit(limit).Offset(offset)
	}
	if err := q.Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// ClearAll удаляет все записи пользователя данной категории.
func (s *PhoneStore[M]) ClearAll(ctx context.Context, userID string) error {
	return s.db.WithContext(ctx).Where("user_id = ?", userID).Delete(new(M)).Error
}

// CountAll возвращает количество записей пользователя данной категории.
func (s *PhoneStore[M]) CountAll(ctx context.Context, userID string) (int64, error) {
	var n int64
	if err := s.db.WithContext(ctx).Model(new(M)).Where("user_id = ?", userID).Count(&n).Error; err != nil {
		return 0, err
	}
	return n, nil
}
