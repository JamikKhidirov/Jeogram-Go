package domain

import "time"

// ChatType distinguishes one-on-one and group conversations.
type ChatType string

const (
	ChatTypePrivate ChatType = "private"
	ChatTypeGroup   ChatType = "group"
)

// Роли участников чата.
const (
	RoleOwner  string = "owner"
	RoleAdmin  string = "admin"
	RoleMember string = "member"
)

// Encryption modes for a chat.
const (
	// EncNone means messages are stored in plaintext on the server.
	EncNone string = "none"
	// EncE2EE means messages are end-to-end encrypted; the server only stores
	// opaque ciphertext and never sees the plaintext.
	EncE2EE string = "e2ee"
)

// Chat is a conversation container.
type Chat struct {
	ID         string    `gorm:"type:uuid;primary_key" json:"id"`
	Type       ChatType  `gorm:"size:16;not null" json:"type"`
	Title      string    `gorm:"size:128" json:"title,omitempty"`
	AvatarURL  string    `gorm:"size:512" json:"avatar_url,omitempty"`
	Encryption string    `gorm:"size:16;not null;default:'none'" json:"encryption"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (Chat) TableName() string { return "chats" }

// ChatParticipant links users to chats.
type ChatParticipant struct {
	ChatID   string    `gorm:"type:uuid;primaryKey" json:"chat_id"`
	UserID   string    `gorm:"type:uuid;primaryKey;index:idx_cp_user_id" json:"user_id"`
	Role     string    `gorm:"size:16;default:'member'" json:"role"`
	Muted    bool      `gorm:"not null;default:false" json:"muted"`
	JoinedAt time.Time `json:"joined_at"`
}

func (ChatParticipant) TableName() string { return "chat_participants" }

// PublicChat is the safe projection of a Chat.
type PublicChat struct {
	ID           string   `json:"id"`
	Type         ChatType `json:"type"`
	Title        string   `json:"title,omitempty"`
	AvatarURL    string   `json:"avatar_url,omitempty"`
	Encryption   string   `json:"encryption"`
	Participants []string `json:"participants,omitempty"`
	CreatedAt    string   `json:"created_at"`
}

// CreatePrivateChatRequest creates a 1:1 conversation with another user.
type CreatePrivateChatRequest struct {
	UserID string `json:"user_id" validate:"required"`
}

// CreateGroupChatRequest creates a group conversation.
type CreateGroupChatRequest struct {
	Title          string   `json:"title" validate:"required,max=128"`
	ParticipantIDs []string `json:"participant_ids" validate:"required,min=1,dive,required"`
	AvatarURL      string   `json:"avatar_url" validate:"omitempty,url,max=512"`
}
