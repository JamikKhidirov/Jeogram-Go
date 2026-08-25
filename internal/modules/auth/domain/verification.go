package domain

import "time"

// VerificationPurpose описывает назначение кода подтверждения.
type VerificationPurpose string

const (
	PurposeEmailVerify   VerificationPurpose = "email_verify"
	PurposePhoneOTP      VerificationPurpose = "phone_otp"
	PurposePasswordReset VerificationPurpose = "password_reset"
	PurposeEmailChange   VerificationPurpose = "email_change"
	PurposePhoneChange   VerificationPurpose = "phone_change"
)

// VerificationCode — одноразовый код (email/phone) с ограниченным сроком.
type VerificationCode struct {
	ID        string              `gorm:"type:uuid;primary_key" json:"id"`
	UserID    string              `gorm:"type:uuid;index" json:"user_id"`
	Target    string              `gorm:"size:255;index" json:"target"`
	Purpose   VerificationPurpose `gorm:"size:32;index" json:"purpose"`
	Code      string              `gorm:"size:16" json:"-"`
	ExpiresAt time.Time           `json:"expires_at"`
	Used      bool                `json:"used"`
	CreatedAt time.Time           `json:"created_at"`
}

func (VerificationCode) TableName() string { return "verification_codes" }
