package domain

import (
	"time"
)

// User is the core account entity. Password hashes are never serialized to JSON.
type User struct {
	ID           string    `gorm:"type:uuid;primary_key" json:"id"`
	Email        string    `gorm:"uniqueIndex;size:255" json:"email"`
	Username     string    `gorm:"uniqueIndex;size:64" json:"username"`
	PasswordHash string    `gorm:"size:255" json:"-"`
	DisplayName  string    `gorm:"size:128" json:"display_name"`
	AvatarURL    string    `gorm:"size:512" json:"avatar_url"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (User) TableName() string { return "users" }

// RegisterRequest is the payload for registration.
type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Username string `json:"username" validate:"required,min=3,max=32"`
	Password string `json:"password" validate:"required,min=8,max=72"`
}

// LoginRequest is the payload for login.
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// RefreshRequest carries a refresh token.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// AuthResult is returned after successful authentication.
type AuthResult struct {
	User         *PublicUser `json:"user"`
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
}

// PublicUser is the safe projection of a User.
type PublicUser struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url"`
	CreatedAt   string `json:"created_at"`
}

// ToPublic converts a User to its public form.
func ToPublic(u *User) *PublicUser {
	return &PublicUser{
		ID:          u.ID,
		Email:       u.Email,
		Username:    u.Username,
		DisplayName: u.DisplayName,
		AvatarURL:   u.AvatarURL,
		CreatedAt:   u.CreatedAt.Format(time.RFC3339),
	}
}
