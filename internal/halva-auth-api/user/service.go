package user

import (
	"context"

	"github.com/pkg/errors"

	"github.com/HalvaPovidlo/halva-services/internal/pkg/user"
)

var (
	ErrNotFound = errors.New("user not found")
)

type cacheService interface {
	Set(item *user.Item)
	Get(id string) (*user.Item, bool)
	All() user.Items
}

type storageService interface {
	Upsert(ctx context.Context, user *user.Item) error
	All(ctx context.Context) (user.Items, error)
}

type service struct {
	cache   cacheService
	storage storageService
}

func New(cache cacheService, storage storageService) *service {
	return &service{
		cache:   cache,
		storage: storage,
	}
}

func (s *service) Upsert(ctx context.Context, id, username, avatar string) error {
	u := &user.Item{
		ID:       id,
		Username: username,
		Avatar:   avatar,
	}
	err := s.storage.Upsert(ctx, u)
	if err != nil {
		return errors.Wrap(err, "upsert user to storage")
	}
	s.cache.Set(u)
	return nil
}

func (s *service) Get(ctx context.Context, id string) (*user.Item, error) {
	if u, ok := s.cache.Get(id); ok {
		return u, nil
	}

	return nil, ErrNotFound
}

func (s *service) All(ctx context.Context) (user.Items, error) {
	cached := s.cache.All()
	if len(cached) != 0 {
		return cached, nil
	}

	users, err := s.storage.All(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "get all users from storage")
	}

	for i := range users {
		s.cache.Set(&users[i])
	}
	return users, nil
}
