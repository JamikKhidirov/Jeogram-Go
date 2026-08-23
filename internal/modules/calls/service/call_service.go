package service

import (
	"context"
	"errors"

	"github.com/jeogram/messenger/internal/modules/calls/domain"
	"github.com/jeogram/messenger/internal/modules/calls/repository"
	chatrepo "github.com/jeogram/messenger/internal/modules/chat/repository"
)

// ErrForbidden indicates the user may not act on the call.
var ErrForbidden = errors.New("forbidden")

// CallService manages call lifecycle.
type CallService struct {
	repo  *repository.CallRepository
	chats *chatrepo.ChatRepository
}

func NewCallService(repo *repository.CallRepository, chats *chatrepo.ChatRepository) *CallService {
	return &CallService{repo: repo, chats: chats}
}

// Start records a new call if the initiator belongs to the chat.
func (s *CallService) Start(ctx context.Context, initiator, chatID string, callType domain.CallType) (*domain.Call, error) {
	ok, err := s.chats.IsParticipant(ctx, chatID, initiator)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrForbidden
	}
	c := &domain.Call{ChatID: chatID, Initiator: initiator, Type: callType}
	if err := s.repo.Create(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

// End terminates a call (only the initiator may end it).
func (s *CallService) End(ctx context.Context, id, userID string) error {
	call, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if call.Initiator != userID {
		return ErrForbidden
	}
	return s.repo.End(ctx, id)
}

// Get возвращает запись звонка по id.
func (s *CallService) Get(ctx context.Context, id string) (*domain.Call, error) {
	return s.repo.Get(ctx, id)
}

// History возвращает историю звонков пользователя.
func (s *CallService) History(ctx context.Context, userID string, limit, offset int) ([]domain.Call, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.repo.ListByUser(ctx, userID, limit, offset)
}
