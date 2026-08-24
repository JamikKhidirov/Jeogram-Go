package domain

import (
	"time"
)

// User is the core account entity. Password hashes are never serialized to JSON.
type User struct {
	ID            string    `gorm:"type:uuid;primary_key" json:"id"`
	Email         string    `gorm:"uniqueIndex;size:255" json:"email"`
	Username      string    `gorm:"uniqueIndex;size:64" json:"username"`
	Phone         string    `gorm:"size:32" json:"phone,omitempty"`
	PasswordHash  string    `gorm:"size:255" json:"-"`
	DisplayName   string    `gorm:"size:128" json:"display_name"`
	AvatarURL     string    `gorm:"size:512" json:"avatar_url"`
	EmailVerified bool      `json:"email_verified"`
	PhoneVerified bool      `json:"phone_verified"`
	Status        string    `gorm:"size:16;default:active" json:"status"`
	Role          string    `gorm:"size:16;default:user" json:"role"`
	LastSeenIP    string    `gorm:"size:64" json:"last_seen_ip,omitempty"`
	UserAgent     string    `gorm:"size:256" json:"user_agent,omitempty"`
	LastSeenAt    time.Time `json:"last_seen_at,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// User statuses and roles.
const (
	StatusActive   = "active"
	StatusBanned   = "banned"
	StatusSuspended = "suspended"
	RoleUser       = "user"
	RoleAdmin      = "admin"
)

func (User) TableName() string { return "users" }

// RegisterRequest is the payload for registration.
type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Username string `json:"username" validate:"required,min=3,max=32"`
	Password string `json:"password" validate:"required,min=8,max=72"`
	Phone    string `json:"phone,omitempty" validate:"omitempty,min=5,max=32"`
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
	ID            string `json:"id"`
	Email         string `json:"email"`
	Username      string `json:"username"`
	DisplayName   string `json:"display_name"`
	AvatarURL     string `json:"avatar_url"`
	Phone         string `json:"phone,omitempty"`
	EmailVerified bool   `json:"email_verified"`
	PhoneVerified bool   `json:"phone_verified"`
	Status        string `json:"status"`
	Role          string `json:"role"`
	CreatedAt     string `json:"created_at"`
}

// ToPublic converts a User to its public form.
func ToPublic(u *User) *PublicUser {
	return &PublicUser{
		ID:            u.ID,
		Email:         u.Email,
		Username:      u.Username,
		DisplayName:   u.DisplayName,
		AvatarURL:     u.AvatarURL,
		Phone:         u.Phone,
		EmailVerified: u.EmailVerified,
		PhoneVerified: u.PhoneVerified,
		Status:        u.Status,
		Role:          u.Role,
		CreatedAt:     u.CreatedAt.Format(time.RFC3339),
	}
}
