package film

import (
	"context"

	"github.com/pkg/errors"

	"github.com/HalvaPovidlo/halva-services/internal/pkg/film"
)

var (
	ErrAlreadyExists = errors.New("film already exists")
	ErrNotFound      = errors.New("film not found")
	ErrNoScore       = errors.New("film has no score from the user")
)

type cacheService interface {
	SetFilm(item *film.Item)
	GetFilm(id string) (*film.Item, bool)
	SetFilms(items film.Items)
	AllFilms() film.Items
}

type storageService interface {
	SetFilm(ctx context.Context, userID string, item *film.Item) error
	AllFilms(ctx context.Context) (film.Items, error)
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

func (s *service) New(ctx context.Context, userID, url string, score film.Score) (*film.Item, error) {
	id := s.kinopoisk.ExtractID(url)
	if _, ok := s.cache.GetFilm(id); ok {
		return nil, ErrAlreadyExists
	}

	f, err := s.kinopoisk.GetFilm(ctx, id)
	if err != nil {
		return nil, errors.New("get film from kinopoisk")
	}

	f.Scores = make(map[string]film.Score, 10)
	f.Scores[userID] = score

	if err := s.storage.SetFilm(ctx, userID, f); err != nil {
		return nil, errors.Wrap(err, "insert film in storage")
	}
	s.cache.SetFilm(f)

	return f, nil
}

func (s *service) Get(ctx context.Context, url string) (*film.Item, error) {
	id := s.kinopoisk.ExtractID(url)
	if f, ok := s.cache.GetFilm(id); ok {
		return f, nil
	}

	// while cache is consistent
	//f, err := s.storage.GetFilm(ctx, id)
	//if err != nil {
	//	return nil, errors.Wrap(err, "get film from storage")
	//}
	//s.cache.SetFilm(f)

	return nil, ErrNotFound
}

func (s *service) All(ctx context.Context) (film.Items, error) {
	cached := s.cache.AllFilms()
	if len(cached) != 0 {
		return cached, nil
	}

	films, err := s.storage.AllFilms(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "get film from storage")
	}
	s.cache.SetFilms(films)

	return films, nil
}

func (s *service) Score(ctx context.Context, userID, url string, score film.Score) (*film.Item, error) {
	id := s.kinopoisk.ExtractID(url)
	cached, ok := s.cache.GetFilm(id)
	if !ok {
		return nil, ErrNotFound
	}

	if len(cached.Scores) == 0 {
		cached.Scores = make(map[string]film.Score, 10)
	}
	cached.Scores[userID] = score

	if err := s.storage.SetFilm(ctx, userID, cached); err != nil {
		return nil, errors.Wrap(err, "insert film to storage")
	}
	s.cache.SetFilm(cached)
	return cached, nil
}

func (s *service) RemoveScore(ctx context.Context, userID, url string) (*film.Item, error) {
	id := s.kinopoisk.ExtractID(url)
	cached, ok := s.cache.GetFilm(id)
	if !ok {
		return nil, ErrNotFound
	}

	if _, ok := cached.Scores[userID]; !ok {
		return nil, ErrNoScore
	}

	delete(cached.Scores, userID)

	if err := s.storage.SetFilm(ctx, userID, cached); err != nil {
		return nil, errors.Wrap(err, "insert film to storage")
	}
	s.cache.SetFilm(cached)
	return cached, nil
}
