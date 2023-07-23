package firestore

import (
	"context"
	"fmt"
	psong "github.com/HalvaPovidlo/halva-services/internal/pkg/song"
)

type storageInterface interface {
	Get(ctx context.Context, id psong.IDType) (*psong.Item, error)
	Set(ctx context.Context, userID string, song *psong.Item) error
	All(ctx context.Context) ([]psong.Item, error)
}

type cacheInterface interface {
	Set(item *psong.Item)
	Get(id psong.IDType) (*psong.Item, bool)
	GetAny(minPlaybacks int64) *psong.Item
}

type service struct {
	storage storageInterface
	cache   cacheInterface
}

func New(storage storageInterface, cache cacheInterface) *service {
	return &service{
		storage: storage,
		cache:   cache,
	}
}

func (s *service) Get(ctx context.Context, id psong.IDType) (*psong.Item, error) {
	item, ok := s.cache.Get(id)
	if ok {
		return item, nil
	}

	item, err := s.storage.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get song from firestore: %+w", err)
	}
	s.cache.Set(item)
	return item, nil
}

func (s *service) Set(ctx context.Context, userID string, item *psong.Item) error {
	err := s.storage.Set(ctx, userID, item)
	if err != nil {
		return fmt.Errorf("set song to firestore: %+w", err)
	}
	s.cache.Set(item)
	return nil
}

func (s *service) GetAny(minPlaybacks int64) *psong.Item {
	return s.cache.GetAny(minPlaybacks)
}

func (s *service) FillCache(ctx context.Context) error {
	all, err := s.storage.All(ctx)
	if err != nil {
		return fmt.Errorf("get all songs from firestore: %+w", err)
	}
	for i := range all {
		s.cache.Set(&all[i])
	}
	return nil
}
