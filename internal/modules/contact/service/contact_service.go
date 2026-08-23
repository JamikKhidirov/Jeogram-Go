package service

import (
	"context"
	"errors"

	authdomain "github.com/jeogram/messenger/internal/modules/auth/domain"
	authrepo "github.com/jeogram/messenger/internal/modules/auth/repository"
	"github.com/jeogram/messenger/internal/modules/contact/domain"
	"github.com/jeogram/messenger/internal/modules/contact/repository"
)

// ContactService реализует сценарии работы с контактами (друзьями).
type ContactService struct {
	contacts *repository.ContactRepository
	users    *authrepo.UserRepository
}

func NewContactService(contacts *repository.ContactRepository, users *authrepo.UserRepository) *ContactService {
	return &ContactService{contacts: contacts, users: users}
}

// Add отправляет запрос в контакты текущим пользователем -> contactID.
func (s *ContactService) Add(ctx context.Context, owner, contactID string) (*domain.Contact, error) {
	if owner == contactID {
		return nil, errors.New("cannot add yourself to contacts")
	}
	if _, err := s.users.GetByID(ctx, contactID); err != nil {
		return nil, err
	}
	return s.contacts.Create(ctx, owner, contactID)
}

// Accept подтверждает входящий запрос (отправитель requester -> я).
func (s *ContactService) Accept(ctx context.Context, owner, requester string) error {
	if owner == requester {
		return errors.New("cannot accept your own request")
	}
	c, err := s.contacts.Get(ctx, requester, owner)
	if err != nil {
		return err
	}
	if c.Status == domain.ContactAccepted {
		return nil
	}
	return s.contacts.Accept(ctx, requester, owner)
}

// Remove удаляет связь с пользователем в любом направлении.
func (s *ContactService) Remove(ctx context.Context, owner, contactID string) error {
	return s.contacts.Delete(ctx, owner, contactID)
}

// List возвращает подтверждённых контактов (публичные профили).
func (s *ContactService) List(ctx context.Context, owner string) ([]authdomain.PublicUser, error) {
	cs, err := s.contacts.ListAccepted(ctx, owner)
	if err != nil {
		return nil, err
	}
	out := make([]authdomain.PublicUser, 0, len(cs))
	for i := range cs {
		u, err := s.users.GetByID(ctx, cs[i].ContactID)
		if err != nil {
			continue
		}
		out = append(out, *authdomain.ToPublic(u))
	}
	return out, nil
}

// ListRequests возвращает публичные профили отправителей входящих запросов.
func (s *ContactService) ListRequests(ctx context.Context, owner string) ([]authdomain.PublicUser, error) {
	cs, err := s.contacts.ListIncomingRequests(ctx, owner)
	if err != nil {
		return nil, err
	}
	out := make([]authdomain.PublicUser, 0, len(cs))
	for i := range cs {
		u, err := s.users.GetByID(ctx, cs[i].OwnerID)
		if err != nil {
			continue
		}
		out = append(out, *authdomain.ToPublic(u))
	}
	return out, nil
}
