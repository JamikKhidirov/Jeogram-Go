package domain

import "time"

// MessageType enumerates supported message kinds.
type MessageType string

const (
	TypeText  MessageType = "text"
	TypeVoice MessageType = "voice"
	TypeImage MessageType = "image"
)

// Message status values.
const (
	// StatusSent is a normally delivered message.
	StatusSent = "sent"
	// StatusScheduled is a message waiting for its scheduled_at time.
	StatusScheduled = "scheduled"
)

// Message is a single chat message.
type Message struct {
	ID              string      `gorm:"type:uuid;primary_key" json:"id"`
	ChatID          string      `gorm:"type:uuid;index:idx_messages_chat_created,priority:1" json:"chat_id"`
	SenderID        string      `gorm:"type:uuid;index" json:"sender_id"`
	Type            MessageType `gorm:"size:16;not null" json:"type"`
	Status          string      `gorm:"size:16;not null;default:sent" json:"status"`
	Text            string      `gorm:"type:text" json:"text,omitempty"`
	MediaURL        string      `gorm:"size:512" json:"media_url,omitempty"`
	ReplyToID       *string     `gorm:"type:uuid" json:"reply_to_id,omitempty"`
	ScheduledAt     *time.Time  `json:"scheduled_at,omitempty"`
	CreatedAt       time.Time   `gorm:"index:idx_messages_chat_created,priority:2" json:"created_at"`
	EditedAt        *time.Time  `json:"edited_at,omitempty"`
	EditVersion     int         `json:"edit_version,omitempty"`
	DeletedAt       *time.Time  `json:"deleted_at,omitempty"`
	DeletedForAllAt *time.Time  `json:"deleted_for_all_at,omitempty"`
	ReadBy          []string    `gorm:"-" json:"read_by,omitempty"`
}

func (Message) TableName() string { return "messages" }

// Reaction — реакция (эмодзи) пользователя на сообщение.
type Reaction struct {
	MessageID string    `gorm:"type:uuid;primaryKey" json:"message_id"`
	UserID    string    `gorm:"type:uuid;primaryKey" json:"user_id"`
	Emoji     string    `gorm:"size:16;primaryKey" json:"emoji"`
	CreatedAt time.Time `json:"created_at"`
}

func (Reaction) TableName() string { return "message_reactions" }

// PinnedMessage — закреплённое сообщение в чате.
type PinnedMessage struct {
	ChatID    string    `gorm:"type:uuid;primaryKey" json:"chat_id"`
	MessageID string    `gorm:"type:uuid;primaryKey" json:"message_id"`
	PinnedBy  string    `gorm:"type:uuid" json:"pinned_by"`
	CreatedAt time.Time `json:"created_at"`
}

func (PinnedMessage) TableName() string { return "pinned_messages" }

// SendMessageRequest is the payload for sending a message.
type SendMessageRequest struct {
	ChatID      string `json:"chat_id" validate:"required"`
	Type        string `json:"type" validate:"required,oneof=text voice image"`
	Text        string `json:"text"`
	MediaURL    string `json:"media_url"`
	ReplyTo     string `json:"reply_to,omitempty"`
	ScheduledAt string `json:"scheduled_at,omitempty"` // RFC3339; if set in the future, message is queued
}

// EditMessageRequest updates the text of a message.
type EditMessageRequest struct {
	Text string `json:"text" validate:"required"`
}

// ReactRequest добавляет реакцию на сообщение.
type ReactRequest struct {
	Emoji string `json:"emoji" validate:"required"`
}

// ForwardRequest пересылает сообщение в другой чат.
type ForwardRequest struct {
	ChatID string `json:"chat_id" validate:"required"`
}

// MarkReadRequest marks a message as read by the caller.
type MarkReadRequest struct {
	MessageIDs []string `json:"message_ids" validate:"required,min=1"`
}

// PublicMessage is the safe projection of a Message.
type PublicMessage struct {
	ID              string            `json:"id"`
	ChatID          string            `json:"chat_id"`
	SenderID        string            `json:"sender_id"`
	Type            MessageType       `json:"type"`
	Text            string            `json:"text,omitempty"`
	MediaURL        string            `json:"media_url,omitempty"`
	ReplyToID       *string           `json:"reply_to_id,omitempty"`
	ReplyPreview    *string           `json:"reply_preview,omitempty"`
	Reactions       []ReactionSummary `json:"reactions,omitempty"`
	Pinned          bool              `json:"pinned,omitempty"`
	CreatedAt       string            `json:"created_at"`
	EditedAt        *string           `json:"edited_at,omitempty"`
	EditVersion     int               `json:"edit_version,omitempty"`
	DeletedForAllAt *string           `json:"deleted_for_all_at,omitempty"`
	ScheduledAt     *string           `json:"scheduled_at,omitempty"`
	Status          string            `json:"status,omitempty"`
	ReadBy          []string          `json:"read_by,omitempty"`
}

// ReactionSummary — агрегированные реакции по эмодзи.
type ReactionSummary struct {
	Emoji string   `json:"emoji"`
	Count int      `json:"count"`
	Users []string `json:"users"`
}

// ReadReceipt records that a user has read a message.
type ReadReceipt struct {
	MessageID string    `gorm:"type:uuid;primaryKey" json:"message_id"`
	UserID    string    `gorm:"type:uuid;primaryKey" json:"user_id"`
	ReadAt    time.Time `json:"read_at"`
}

func (ReadReceipt) TableName() string { return "message_reads" }
