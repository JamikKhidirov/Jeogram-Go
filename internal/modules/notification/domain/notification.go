package domain

import "time"

// Platform identifies the target push platform.
type Platform string

const (
	PlatformIOS     Platform = "ios"
	PlatformAndroid Platform = "android"
	PlatformWeb     Platform = "web"
)

// DeviceMeta carries client/device telemetry captured at registration time.
type DeviceMeta struct {
	DeviceModel string
	OSVersion   string
	AppVersion  string
	Locale      string
	Timezone    string
	IP          string
	UserAgent   string
}

// DeviceToken stores a push registration and telemetry for a user's device.
type DeviceToken struct {
	UserID      string    `gorm:"type:uuid;primaryKey" json:"user_id"`
	Platform    Platform  `gorm:"size:16;primaryKey" json:"platform"`
	Token       string    `gorm:"size:512" json:"-"`
	DeviceModel string    `gorm:"size:128" json:"device_model,omitempty"`
	OSVersion   string    `gorm:"size:64" json:"os_version,omitempty"`
	AppVersion  string    `gorm:"size:64" json:"app_version,omitempty"`
	Locale      string    `gorm:"size:16" json:"locale,omitempty"`
	Timezone    string    `gorm:"size:64" json:"timezone,omitempty"`
	LastIP      string    `gorm:"size:64" json:"last_ip,omitempty"`
	UserAgent   string    `gorm:"size:256" json:"user_agent,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (DeviceToken) TableName() string { return "device_tokens" }

// RegisterDeviceRequest is the payload for registering a push token.
type RegisterDeviceRequest struct {
	Platform    Platform `json:"platform" validate:"required,oneof=ios android web"`
	Token       string   `json:"token" validate:"required"`
	DeviceModel string   `json:"device_model,omitempty"`
	OSVersion   string   `json:"os_version,omitempty"`
	AppVersion  string   `json:"app_version,omitempty"`
	Locale      string   `json:"locale,omitempty"`
	Timezone    string   `json:"timezone,omitempty"`
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
