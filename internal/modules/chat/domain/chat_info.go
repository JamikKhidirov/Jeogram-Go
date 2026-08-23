package domain

import "time"

type ChatInfo struct {
	ID            string
	Name          string
	Description   string
	AvatarURL     string
	Type          string // private, group
	CreatedAt     time.Time
	UpdatedAt     time.Time
	MemberCount   int64
	MessageCount  int64
	CreatedBy     string
}

type ChatStats struct {
	ChatID          string
	MemberCount     int64
	MessageCount    int64
	LastMessageAt   time.Time
	AverageResponse time.Duration
}

type UpdateChatInfoRequest struct {
	Name        string `json:"name" validate:"required,min=1,max=100"`
	Description string `json:"description" validate:"max=500"`
	AvatarURL   string `json:"avatar_url" validate:"url"`
}
