package domain

import "time"

// ContactStatus — статус связи между пользователями.
type ContactStatus string

const (
	// ContactPending — входящий запрос в контакты, ожидает подтверждения.
	ContactPending ContactStatus = "pending"
	// ContactAccepted — связь подтверждена, пользователь в списке контактов.
	ContactAccepted ContactStatus = "accepted"
)

// Contact представляет связь «владелец -> контакт» (запрос или подтверждённый контакт).
type Contact struct {
	ID        string        `gorm:"type:uuid;primary_key" json:"id"`
	OwnerID   string        `gorm:"type:uuid;index" json:"owner_id"`
	ContactID string        `gorm:"type:uuid;index" json:"contact_id"`
	Status    ContactStatus `gorm:"size:16;default:'pending'" json:"status"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

func (Contact) TableName() string { return "contacts" }

// AddContactRequest — тело запроса на добавление в контакты.
type AddContactRequest struct {
	UserID string `json:"user_id" validate:"required"`
}
