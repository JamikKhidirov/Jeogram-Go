package domain

import "time"

// UserSettings stores per-user preferences.
type UserSettings struct {
	UserID               string    `gorm:"type:uuid;primary_key" json:"user_id"`
	Theme                string    `gorm:"size:16;default:'system'" json:"theme"`
	Language             string    `gorm:"size:8;default:'en'" json:"language"`
	NotificationsEnabled bool      `gorm:"default:true" json:"notifications_enabled"`
	OnlineVisible        bool      `gorm:"default:true" json:"online_visible"`
	UpdatedAt            time.Time `json:"updated_at"`
}

func (UserSettings) TableName() string { return "user_settings" }

// UpdateSettingsRequest is the editable settings payload.
type UpdateSettingsRequest struct {
	Theme                string `json:"theme" validate:"omitempty,oneof=system light dark"`
	Language             string `json:"language" validate:"omitempty,len=2"`
	NotificationsEnabled *bool  `json:"notifications_enabled"`
	OnlineVisible        *bool  `json:"online_visible"`
}

// UpdateProfileRequest is the editable profile payload.
type UpdateProfileRequest struct {
	DisplayName string `json:"display_name" validate:"omitempty,max=128"`
	AvatarURL   string `json:"avatar_url" validate:"omitempty,url,max=512"`
	Username    string `json:"username" validate:"omitempty,min=3,max=32"`
}

// BlockedUser представляет факт блокировки одного пользователя другим.
type BlockedUser struct {
	BlockerID string    `gorm:"type:uuid;primaryKey" json:"blocker_id"`
	BlockedID string    `gorm:"type:uuid;primaryKey" json:"blocked_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (BlockedUser) TableName() string { return "blocked_users" }

// BlockRequest — тело запроса на блокировку.
type BlockRequest struct {
	UserID string `json:"user_id" validate:"required"`
}
