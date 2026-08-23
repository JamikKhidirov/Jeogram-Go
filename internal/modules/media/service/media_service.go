package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/jeogram/messenger/internal/config"
	"github.com/jeogram/messenger/internal/modules/media/domain"
)

// MediaService stores uploaded files on local disk and returns public URLs.
type MediaService struct {
	cfg config.MediaConfig
}

func NewMediaService(cfg config.MediaConfig) (*MediaService, error) {
	if err := os.MkdirAll(cfg.UploadDir, 0o755); err != nil {
		return nil, err
	}
	return &MediaService{cfg: cfg}, nil
}

// Save validates and persists an uploaded file, returning its public URL.
func (s *MediaService) Save(mediaType domain.MediaType, contentType, filename string, data []byte) (string, error) {
	if !allowed(s.AllowedTypes(mediaType), contentType) {
		return "", fmt.Errorf("unsupported content type %q for %s", contentType, mediaType)
	}
	ext := strings.ToLower(filepath.Ext(filename))
	if !allowed(s.AllowedExts(mediaType), ext) {
		return "", fmt.Errorf("unsupported extension %q for %s", ext, mediaType)
	}
	if int64(len(data)) > s.cfg.MaxFileSize {
		return "", fmt.Errorf("file too large: max %d bytes", s.cfg.MaxFileSize)
	}

	// Hash content to avoid duplicates and provide a stable name.
	sum := sha256.Sum256(data)
	stored := fmt.Sprintf("%s_%s%s", uuid.NewString()[:8], hex.EncodeToString(sum[:])[:16], ext)
	dir := filepath.Join(s.cfg.UploadDir, string(mediaType))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	dst := filepath.Join(dir, stored)
	if err := os.WriteFile(dst, data, 0o600); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%s/%s", strings.TrimRight(s.cfg.BaseURL, "/"), mediaType, stored), nil
}

// PathFor resolves a stored URL back to a filesystem path (for serving).
func (s *MediaService) PathFor(url string) (string, error) {
	trimmed := strings.TrimPrefix(url, strings.TrimRight(s.cfg.BaseURL, "/")+"/")
	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid media url")
	}
	p := filepath.Join(s.cfg.UploadDir, filepath.Clean(parts[0]), filepath.Clean(parts[1]))
	if !strings.HasPrefix(p, filepath.Clean(s.cfg.UploadDir)) {
		return "", fmt.Errorf("path traversal denied")
	}
	return p, nil
}

func (s *MediaService) AllowedTypes(m domain.MediaType) []string {
	return domain.AllowedContentTypes[m]
}
func (s *MediaService) AllowedExts(m domain.MediaType) []string { return domain.AllowedExtensions[m] }
func (s *MediaService) BaseURL() string                         { return s.cfg.BaseURL }

func allowed(list []string, val string) bool {
	for _, v := range list {
		if strings.EqualFold(v, val) {
			return true
		}
	}
	return false
}
