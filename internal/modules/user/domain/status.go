package domain

import (
	"time"
)

type UserStatus string

const (
	StatusOnline    UserStatus = "online"
	StatusOffline   UserStatus = "offline"
	StatusAway      UserStatus = "away"
	StatusBusy      UserStatus = "busy"
	StatusDND       UserStatus = "dnd" // Do Not Disturb
)

type UserStatusModel struct {
	ID        string
	UserID    string
	Status    UserStatus
	UpdatedAt time.Time
}

type UserSettingsModel struct {
	ID                  string
	UserID              string
	NotificationsEnabled bool
	ReadReceipts        bool
	TypingIndicators    bool
	OnlineStatus        bool
	Theme               string // light | dark | auto
	Language            string // en | ru
	Privacy             string // public | private | contacts_only
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type UpdateStatusRequest struct {
	Status UserStatus `json:"status" validate:"required,oneof=online offline away busy dnd"`
}

type UpdateSettingsRequest struct {
	NotificationsEnabled *bool  `json:"notifications_enabled"`
	ReadReceipts         *bool  `json:"read_receipts"`
	TypingIndicators     *bool  `json:"typing_indicators"`
	OnlineStatus         *bool  `json:"online_status"`
	Theme                *string `json:"theme" validate:"omitempty,oneof=light dark auto"`
	Language             *string `json:"language" validate:"omitempty,oneof=en ru"`
	Privacy              *string `json:"privacy" validate:"omitempty,oneof=public private contacts_only"`
}

type BlockUserRequest struct {
	BlockedUserID string `json:"blocked_user_id" validate:"required"`
	Reason        string `json:"reason"`
}

type ReportUserRequest struct {
	ReportedUserID string `json:"reported_user_id" validate:"required"`
	Reason         string `json:"reason" validate:"required,min=10"`
}
