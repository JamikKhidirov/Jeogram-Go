package domain

// MediaType enumerates supported upload categories.
type MediaType string

const (
	MediaImage MediaType = "image"
	MediaVoice MediaType = "voice"
)

// AllowedContentTypes maps a media type to acceptable MIME types.
var AllowedContentTypes = map[MediaType][]string{
	MediaImage: {"image/jpeg", "image/png", "image/gif", "image/webp"},
	MediaVoice: {"audio/mpeg", "audio/wav", "audio/ogg", "audio/webm", "audio/x-m4a"},
}

// AllowedExtensions maps a media type to acceptable file extensions.
var AllowedExtensions = map[MediaType][]string{
	MediaImage: {".jpg", ".jpeg", ".png", ".gif", ".webp"},
	MediaVoice: {".mp3", ".wav", ".ogg", ".webm", ".m4a"},
}
