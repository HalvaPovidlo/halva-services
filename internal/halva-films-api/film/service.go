package film

import (
	"context"

	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/HalvaPovidlo/halva-services/internal/pkg/film"
)

var (
	ErrAlreadyExists = errors.New("film already exists")
	ErrNotFound      = errors.New("film not found")
	ErrNoScore       = errors.New("film has no score from the user")
)

type cacheService interface {
	Set(item *film.Item)
	Get(id string) (*film.Item, bool)
	SetAll(items film.Items)
	All() film.Items
	User(userID string) (film.Items, bool)
	SetUser(userID string, items film.Items)
}

type storageService interface {
	Set(ctx context.Context, userID string, item *film.Item) error
	All(ctx context.Context) (film.Items, error)
	User(ctx context.Context, userID string) ([]string, error)
}

type kinopoisk interface {
	GetFilm(ctx context.Context, url string) (*film.Item, error)
	ExtractID(uri string) string
}

type service struct {
	cache     cacheService
	storage   storageService
	kinopoisk kinopoisk
}

func New(kinopoisk kinopoisk, cache cacheService, storage storageService) *service {
	return &service{
		cache:     cache,
		storage:   storage,
		kinopoisk: kinopoisk,
	}
}

func (s *service) FillCache(ctx context.Context) error {
	_, err := s.All(ctx)
	return err
}

func (s *service) New(ctx context.Context, userID, url string, score film.Score) (*film.Item, error) {
	id := s.kinopoisk.ExtractID(url)
	if _, ok := s.cache.Get(id); ok {
		return nil, ErrAlreadyExists
	}

	f, err := s.kinopoisk.GetFilm(ctx, id)
	if err != nil {
		return nil, errors.New("get film from kinopoisk")
	}

	f.Scores = make(map[string]film.Score, 10)
	f.Scores[userID] = score

	if err := s.storage.Set(ctx, userID, f); err != nil {
		return nil, errors.Wrap(err, "insert film in storage")
	}
	s.cache.Set(f)

	return f, nil
}

func (s *service) Get(ctx context.Context, url string) (*film.Item, error) {
	id := s.kinopoisk.ExtractID(url)
	if f, ok := s.cache.Get(id); ok {
		return f, nil
	}

	return nil, ErrNotFound
}

func (s *service) All(ctx context.Context) (film.Items, error) {
	cached := s.cache.All()
	if len(cached) != 0 {
		return cached, nil
	}

	films, err := s.storage.All(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "get film from storage")
	}
	s.cache.SetAll(films)

	return films, nil
}

func (s *service) Score(ctx context.Context, userID, url string, score film.Score) (*film.Item, error) {
	id := s.kinopoisk.ExtractID(url)
	cached, ok := s.cache.Get(id)
	if !ok {
		return nil, ErrNotFound
	}

	if len(cached.Scores) == 0 {
		cached.Scores = make(map[string]film.Score, 10)
	}
	cached.Scores[userID] = score

	if err := s.storage.Set(ctx, userID, cached); err != nil {
		return nil, errors.Wrap(err, "insert film to storage")
	}
	s.cache.Set(cached)
	return cached, nil
}

func (s *service) RemoveScore(ctx context.Context, userID, url string) (*film.Item, error) {
	id := s.kinopoisk.ExtractID(url)
	cached, ok := s.cache.Get(id)
	if !ok {
		return nil, ErrNotFound
	}

	if _, ok := cached.Scores[userID]; !ok {
		return nil, ErrNoScore
	}

	delete(cached.Scores, userID)

	if err := s.storage.Set(ctx, userID, cached); err != nil {
		return nil, errors.Wrap(err, "insert film to storage")
	}
	s.cache.Set(cached)
	return cached, nil
}

func (s *service) User(ctx context.Context, userID string) (film.Items, error) {
	cached, ok := s.cache.User(userID)
	if ok {
		return cached, nil
	}

	filmsID, err := s.storage.User(ctx, userID)
	switch {
	case status.Code(err) == codes.NotFound:
		return nil, ErrNotFound
	case err != nil:
		return nil, errors.Wrap(err, "get user films id from storage")
	}

	userFilms := make(film.Items, 0, len(filmsID))
	for i := range filmsID {
		if f, err := s.Get(ctx, filmsID[i]); err == nil {
			userFilms = append(userFilms, *f)
		}
	}

	s.cache.SetUser(userID, userFilms)
	return userFilms, nil
}
