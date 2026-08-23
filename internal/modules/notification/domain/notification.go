package domain

import "time"

// Platform identifies the target push platform.
type Platform string

const (
	PlatformIOS     Platform = "ios"
	PlatformAndroid Platform = "android"
	PlatformWeb     Platform = "web"
)

// DeviceToken stores a push registration for a user on a specific device.
type DeviceToken struct {
	UserID    string    `gorm:"type:uuid;primaryKey" json:"user_id"`
	Platform  Platform  `gorm:"size:16;primaryKey" json:"platform"`
	Token     string    `gorm:"size:512" json:"-"`
	CreatedAt time.Time `json:"created_at"`
}

func (DeviceToken) TableName() string { return "device_tokens" }

// RegisterDeviceRequest is the payload for registering a push token.
type RegisterDeviceRequest struct {
	Platform Platform `json:"platform" validate:"required,oneof=ios android web"`
	Token    string   `json:"token" validate:"required"`
}

// PushPayload is the notification content sent to devices.
type PushPayload struct {
	UserID    string
	Title     string
	Body      string
	ChatID    string
	MessageID string
	Type      string
}

// Notification is an in-app notification persisted for a user.
type Notification struct {
	ID        string    `gorm:"type:uuid;primary_key" json:"id"`
	UserID    string    `gorm:"type:uuid;index" json:"user_id"`
	ChatID    string    `gorm:"type:uuid" json:"chat_id"`
	MessageID string    `gorm:"type:uuid" json:"message_id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Type      string    `json:"type"`
	Read      bool      `gorm:"default:false" json:"read"`
	CreatedAt time.Time `json:"created_at"`
}

func (Notification) TableName() string { return "notifications" }

// PublicNotification is the safe projection of a Notification.
type PublicNotification struct {
	ID        string `json:"id"`
	ChatID    string `json:"chat_id"`
	MessageID string `json:"message_id"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Type      string `json:"type"`
	Read      bool   `json:"read"`
	CreatedAt string `json:"created_at"`
}
