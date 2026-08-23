package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jeogram/messenger/internal/modules/message/domain"
	"github.com/jeogram/messenger/internal/modules/message/repository"
)

var ErrMessageNotFound = errors.New("message not found")

type MessageExtendedService struct {
	repo *repository.MessageRepository
}

func NewMessageExtendedService(repo *repository.MessageRepository) *MessageExtendedService {
	return &MessageExtendedService{repo: repo}
}

func (s *MessageExtendedService) Quote(ctx context.Context, userID, messageID, text string) (*domain.Message, error) {
	original, err := s.repo.GetByID(ctx, messageID)
	if err != nil {
		return nil, err
	}

	new := &domain.Message{
		ID:        fmt.Sprintf("msg_%d", time.Now().UnixNano()),
		ChatID:    original.ChatID,
		UserID:    userID,
		Text:      text,
		Type:      "text",
		ReplyTo:   messageID,
		Status:    "sent",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.repo.Create(ctx, new); err != nil {
		return nil, err
	}
	return new, nil
}

func (s *MessageExtendedService) ListUnsent(ctx context.Context, userID string) ([]domain.Message, error) {
	return s.repo.ListByStatus(ctx, userID, "pending")
}
