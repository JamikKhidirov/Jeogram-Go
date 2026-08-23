package domain

import "time"

type Message struct {
	ID         string
	ChatID     string
	UserID     string
	Text       string
	Type       string // text, image, voice
	MediaURL   string
	ReplyTo    string    // message ID this replies to
	Status     string    // sent, delivered, read
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type SendMessageRequest struct {
	ChatID  string `json:"chat_id" validate:"required"`
	Text    string `json:"text" validate:"required,max=4096"`
	Type    string `json:"type" validate:"required,oneof=text image voice"`
	MediaURL string `json:"media_url" validate:"omitempty,url"`
	ReplyTo string `json:"reply_to" validate:"omitempty"`
}

type QuoteRequest struct {
	Text string `json:"text" validate:"required,max=4096"`
}
