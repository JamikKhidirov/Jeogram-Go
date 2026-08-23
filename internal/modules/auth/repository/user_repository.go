package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jeogram/messenger/internal/modules/auth/domain"
	"gorm.io/gorm"
)

// ErrUserNotFound is returned when a user cannot be located.
var ErrUserNotFound = errors.New("user not found")

// ErrDuplicateEmail / ErrDuplicateUsername indicate a uniqueness violation.
var ErrDuplicateEmail = errors.New("email already in use")
var ErrDuplicateUsername = errors.New("username already in use")

// UserRepository persists and loads users.
type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, u *domain.User) error {
	u.ID = uuid.NewString()
	if err := r.db.WithContext(ctx).Create(u).Error; err != nil {
		if isUniqueViolation(err) {
			if contains(err.Error(), "users_email_key") {
				return ErrDuplicateEmail
			}
			return ErrDuplicateUsername
		}
		return err
	}
	return nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	var u domain.User
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	var u domain.User
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	var u domain.User
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) Update(ctx context.Context, u *domain.User) error {
	return r.db.WithContext(ctx).Save(u).Error
}

// Delete удаляет аккаунт пользователя по id.
func (r *UserRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&domain.User{}).Error
}

func (r *UserRepository) Search(ctx context.Context, query string, limit int) ([]domain.User, error) {
	var users []domain.User
	like := "%" + query + "%"
	// LOWER(...) LIKE LOWER(...) работает и в PostgreSQL, и в SQLite.
	err := r.db.WithContext(ctx).
		Where("LOWER(username) LIKE LOWER(?) OR LOWER(display_name) LIKE LOWER(?)", like, like).
		Limit(limit).Find(&users).Error
	return users, err
}

func isUniqueViolation(err error) bool {
	return err != nil && contains(err.Error(), "duplicate key value")
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
