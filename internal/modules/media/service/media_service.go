package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jeogram/messenger/internal/config"
	"github.com/jeogram/messenger/internal/modules/media/domain"
	"github.com/jeogram/messenger/internal/modules/media/repository"
	"gorm.io/gorm"
)

// MediaService stores uploaded files on local disk and tracks metadata by UUID.
type MediaService struct {
	cfg  config.MediaConfig
	repo *repository.MediaRepository
}

// NewMediaService builds a MediaService.
func NewMediaService(cfg config.MediaConfig, db *gorm.DB) (*MediaService, error) {
	if err := os.MkdirAll(cfg.UploadDir, 0o755); err != nil {
		return nil, err
	}
	return &MediaService{cfg: cfg, repo: repository.NewMediaRepository(db)}, nil
}

// Save validates and persists an uploaded file, returning its metadata (with id).
func (s *MediaService) Save(ctx context.Context, ownerID string, mediaType domain.MediaType, contentType, filename string, data []byte) (*domain.MediaRecord, error) {
	if !allowed(s.AllowedTypes(mediaType), contentType) {
		return nil, fmt.Errorf("unsupported content type %q for %s", contentType, mediaType)
	}
	ext := strings.ToLower(filepath.Ext(filename))
	if !allowed(s.AllowedExts(mediaType), ext) {
		return nil, fmt.Errorf("unsupported extension %q for %s", ext, mediaType)
	}
	if int64(len(data)) > s.cfg.MaxFileSize {
		return nil, fmt.Errorf("file too large: max %d bytes", s.cfg.MaxFileSize)
	}

	id := uuid.NewString()
	stored := fmt.Sprintf("%s%s", id, ext)
	dir := filepath.Join(s.cfg.UploadDir, string(mediaType))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	dst := filepath.Join(dir, stored)
	if err := os.WriteFile(dst, data, 0o600); err != nil {
		return nil, err
	}
	url := fmt.Sprintf("%s/%s/%s", strings.TrimRight(s.cfg.BaseURL, "/"), mediaType, stored)
	rec := &domain.MediaRecord{
		OwnerID:     ownerID,
		Type:        mediaType,
		ContentType: contentType,
		Filename:    filename,
		URL:         url,
		Size:        int64(len(data)),
		CreatedAt:   time.Now(),
	}
	if s.repo != nil {
		if err := s.repo.Create(ctx, rec); err != nil {
			return nil, err
		}
	}
	return rec, nil
}

// Get returns media metadata by id.
func (s *MediaService) Get(ctx context.Context, id string) (*domain.MediaRecord, error) {
	if s.repo == nil {
		return nil, repository.ErrMediaNotFound
	}
	return s.repo.Get(ctx, id)
}

// Delete removes the media record and the underlying file.
func (s *MediaService) Delete(ctx context.Context, id string) error {
	if s.repo == nil {
		return repository.ErrMediaNotFound
	}
	rec, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if p, err := s.PathFor(rec.URL); err == nil {
		_ = os.Remove(p)
	}
	return s.repo.Delete(ctx, id)
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

// AllowedTypes returns the allowed MIME types for a media type.
func (s *MediaService) AllowedTypes(m domain.MediaType) []string {
	return domain.AllowedContentTypes[m]
}

// AllowedExts returns the allowed file extensions for a media type.
func (s *MediaService) AllowedExts(m domain.MediaType) []string { return domain.AllowedExtensions[m] }

// BaseURL returns the configured public base URL for media.
func (s *MediaService) BaseURL() string { return s.cfg.BaseURL }

func allowed(list []string, val string) bool {
	for _, v := range list {
		if strings.EqualFold(v, val) {
			return true
		}
	}
	return false
}
