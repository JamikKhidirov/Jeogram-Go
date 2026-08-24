package domain

import "time"

// PreKey is a one-time prekey (Signal-style) uploaded by a client. The server
// stores the public material opaquely and hands out a single unused key per
// request, implementing the "prekey bundle" part of an E2EE handshake.
type PreKey struct {
	ID           string    `gorm:"type:uuid;primary_key" json:"id"`
	UserID       string    `gorm:"type:uuid;index:idx_e2ee_prekeys_user" json:"user_id"`
	KeyID        string    `gorm:"size:64" json:"key_id"`
	PublicKey    string    `gorm:"type:text" json:"public_key"`
	SignatureKey string    `gorm:"type:text" json:"signature_key"`
	Used         bool      `gorm:"not null;default:false" json:"used"`
	CreatedAt    time.Time `json:"created_at"`
}

func (PreKey) TableName() string { return "e2ee_prekeys" }

// PreKeyUpload is the payload for uploading a batch of prekeys.
type PreKeyUpload struct {
	PreKeys []PreKeyItem `json:"prekeys" validate:"required,min=1,dive"`
}

// PreKeyItem is a single prekey in an upload.
type PreKeyItem struct {
	KeyID        string `json:"key_id" validate:"required"`
	PublicKey    string `json:"public_key" validate:"required"`
	SignatureKey string `json:"signature_key" validate:"required"`
}
