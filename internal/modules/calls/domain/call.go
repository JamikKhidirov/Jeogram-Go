package domain

import "time"

// CallType distinguishes audio and video calls.
type CallType string

const (
	CallAudio CallType = "audio"
	CallVideo CallType = "video"
)

// CallStatus tracks the lifecycle of a call.
type CallStatus string

const (
	CallStatusActive CallStatus = "active"
	CallStatusEnded  CallStatus = "ended"
)

// Call is a record of a realtime call in a chat.
type Call struct {
	ID        string     `gorm:"type:uuid;primary_key" json:"id"`
	ChatID    string     `gorm:"type:uuid;index" json:"chat_id"`
	Initiator string     `gorm:"type:uuid" json:"initiator"`
	Type      CallType   `gorm:"size:16" json:"type"`
	Status        CallStatus `gorm:"size:16" json:"status"`
	StartedAt     time.Time  `json:"started_at"`
	EndedAt       *time.Time `json:"ended_at,omitempty"`
	RecordingURL  string     `json:"recording_url,omitempty"`
	RecordedAt    *time.Time `json:"recorded_at,omitempty"`
}

func (Call) TableName() string { return "calls" }

// SignalMessage is exchanged over the call websocket for WebRTC negotiation.
type SignalMessage struct {
	Type    string      `json:"type"` // offer | answer | ice | call | hangup
	ChatID  string      `json:"chat_id"`
	To      string      `json:"to,omitempty"`
	Payload interface{} `json:"payload,omitempty"`
}
