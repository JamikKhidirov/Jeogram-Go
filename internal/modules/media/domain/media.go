package domain

// MediaType enumerates supported upload categories.
type MediaType string

const (
	MediaImage    MediaType = "image"
	MediaVoice    MediaType = "voice"
	MediaVideo    MediaType = "video"
	MediaDocument MediaType = "document"
)

// AllowedContentTypes maps a media type to acceptable MIME types.
var AllowedContentTypes = map[MediaType][]string{
	MediaImage:    {"image/jpeg", "image/png", "image/gif", "image/webp"},
	MediaVoice:    {"audio/mpeg", "audio/wav", "audio/ogg", "audio/webm", "audio/x-m4a"},
	MediaVideo:    {"video/mp4", "video/webm", "video/quicktime", "video/x-matroska"},
	MediaDocument: {"application/pdf", "application/msword", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "application/vnd.ms-excel", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "application/vnd.ms-powerpoint", "application/vnd.openxmlformats-officedocument.presentationml.presentation", "text/plain", "application/zip", "application/octet-stream"},
}

// AllowedExtensions maps a media type to acceptable file extensions.
var AllowedExtensions = map[MediaType][]string{
	MediaImage:    {".jpg", ".jpeg", ".png", ".gif", ".webp"},
	MediaVoice:    {".mp3", ".wav", ".ogg", ".webm", ".m4a"},
	MediaVideo:    {".mp4", ".webm", ".mov", ".mkv"},
	MediaDocument: {".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".txt", ".zip"},
}
