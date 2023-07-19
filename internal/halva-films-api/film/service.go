package film

import (
	"context"
	"fmt"
	"time"

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
	User(userID string) ([]string, bool)
	SetUser(userID string, filmsID []string)
	UserAdd(userID string, filmID string)
	UserRemove(userID string, filmID string)
}

type storageService interface {
	Set(ctx context.Context, userID string, item *film.Item) error
	All(ctx context.Context) (film.Items, error)
	User(ctx context.Context, userID string) ([]string, error)
	Comments(ctx context.Context, filmID string) ([]film.Comment, error)
	AddComment(ctx context.Context, filmID string, comment *film.Comment) error
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
	films, err := s.All(ctx)
	users := make(map[string][]string)
	for i := range films {
		for userID, _ := range films[i].Scores {
			users[userID] = append(users[userID], films[i].ID)
		}
	}

	for userID, films := range users {
		s.cache.SetUser(userID, films)
	}

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
	s.cache.UserAdd(userID, f.ID)
	return f, nil
}

func (s *service) Get(ctx context.Context, url string) (*film.Item, error) {
	return s.get(ctx, url, true)
}

func (s *service) get(ctx context.Context, url string, withComments bool) (*film.Item, error) {
	id := s.kinopoisk.ExtractID(url)
	f, ok := s.cache.Get(id)
	if !ok {
		return nil, ErrNotFound
	}
	if f.NoComments || !withComments || len(f.Comments) > 0 {
		return f, nil
	}

	comments, err := s.storage.Comments(ctx, f.ID)
	if err != nil {
		return nil, fmt.Errorf("get comments from storage: %+w", err)
	}
	f.Comments = comments
	f.NoComments = len(f.Comments) == 0

	s.cache.Set(f)
	return f, nil
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
	s.cache.UserAdd(userID, cached.ID)
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
	s.cache.UserRemove(userID, cached.ID)
	return cached, nil
}

func (s *service) Comment(ctx context.Context, userID, url, text string) (*film.Item, error) {
	f, err := s.get(ctx, url, true)
	if err != nil {
		return nil, err
	}

	if len(f.Comments) == 0 {
		f.Comments = make([]film.Comment, 0, 10)
	}

	comment := film.Comment{
		UserID:    userID,
		Text:      text,
		CreatedAt: time.Now(),
	}
	f.Comments = append(f.Comments, comment)
	f.NoComments = false

	if err := s.storage.AddComment(ctx, f.ID, &comment); err != nil {
		return nil, fmt.Errorf("add comment to storage: %+w", err)
	}
	s.cache.Set(f)
	return f, nil
}

func (s *service) User(ctx context.Context, userID string) (film.Items, error) {
	var err error
	filmsID, ok := s.cache.User(userID)
	if !ok {
		filmsID, err = s.storage.User(ctx, userID)
		switch {
		case status.Code(err) == codes.NotFound:
			return nil, ErrNotFound
		case err != nil:
			return nil, errors.Wrap(err, "get user films id from storage")
		}
	}
	s.cache.SetUser(userID, filmsID)

	userFilms := make(film.Items, 0, len(filmsID))
	for i := range filmsID {
		if f, err := s.get(ctx, filmsID[i], false); err == nil {
			userFilms = append(userFilms, *f)
		}
	}

	return userFilms, nil
}
