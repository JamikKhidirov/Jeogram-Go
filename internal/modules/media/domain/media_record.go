package domain

import "time"

// MediaRecord is metadata for an uploaded file, keyed by a stable UUID so it
// can be referenced, downloaded and deleted via /media/{id} endpoints.
type MediaRecord struct {
	ID          string    `gorm:"type:uuid;primary_key" json:"id"`
	OwnerID     string    `gorm:"type:uuid;index" json:"owner_id"`
	Type        MediaType `gorm:"size:16;not null" json:"type"`
	ContentType string    `gorm:"size:128" json:"content_type"`
	Filename    string    `gorm:"size:255" json:"filename"`
	URL         string    `gorm:"size:512" json:"url"`
	Size        int64     `json:"size"`
	CreatedAt   time.Time `json:"created_at"`
}

// TableName overrides the default GORM table name.
func (MediaRecord) TableName() string { return "media" }
