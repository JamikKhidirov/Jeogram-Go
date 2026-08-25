package service

import (
	"context"
	"errors"

	authdomain "github.com/jeogram/messenger/internal/modules/auth/domain"
	authrepo "github.com/jeogram/messenger/internal/modules/auth/repository"
	"github.com/jeogram/messenger/internal/modules/contact/domain"
	"github.com/jeogram/messenger/internal/modules/contact/repository"
	userrepo "github.com/jeogram/messenger/internal/modules/user/repository"
)

// ContactService реализует сценарии работы с контактами (друзьями).
type ContactService struct {
	contacts *repository.ContactRepository
	users    *authrepo.UserRepository
	settings *userrepo.SettingsRepository
}

func NewContactService(contacts *repository.ContactRepository, users *authrepo.UserRepository, settings *userrepo.SettingsRepository) *ContactService {
	return &ContactService{contacts: contacts, users: users, settings: settings}
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

// Get возвращает запись контакта между владельцем и пользователем.
func (s *ContactService) Get(ctx context.Context, owner, contactID string) (*domain.Contact, error) {
	return s.contacts.Get(ctx, owner, contactID)
}

// Sync массово добавляет контакты из телефонной книги (по списку user_id).
// Возвращает число реально добавленных (новых) контактов.
func (s *ContactService) Sync(ctx context.Context, owner string, contactIDs []string) (int, error) {
	added := 0
	for _, cid := range contactIDs {
		if cid == "" || cid == owner {
			continue
		}
		if _, err := s.users.GetByID(ctx, cid); err != nil {
			continue
		}
		if _, err := s.contacts.Create(ctx, owner, cid); err != nil {
			// уже существует или ошибка — пропускаем
			continue
		}
		added++
	}
	return added, nil
}

// Block блокирует пользователя (привязано к чёрному списку аккаунта).
func (s *ContactService) Block(ctx context.Context, blocker, blocked string) error {
	if blocker == blocked {
		return errors.New("cannot block yourself")
	}
	if _, err := s.users.GetByID(ctx, blocked); err != nil {
		return err
	}
	return s.settings.Block(ctx, blocker, blocked)
}

// ListIDs возвращает id всех подтверждённых контактов пользователя.
func (s *ContactService) ListIDs(ctx context.Context, owner string) ([]string, error) {
	return s.contacts.ListIDs(ctx, owner)
}
