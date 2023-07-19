package film

import (
	"context"
	"fmt"

	"cloud.google.com/go/firestore"
	"github.com/pkg/errors"
	"google.golang.org/api/iterator"

	"github.com/HalvaPovidlo/halva-services/internal/pkg/film"
	"github.com/HalvaPovidlo/halva-services/internal/pkg/user"
	fire "github.com/HalvaPovidlo/halva-services/pkg/firestore"
)

const approximateFilmsNumber = 512

type storage struct {
	*firestore.Client
}

func NewStorage(client *firestore.Client) *storage {
	return &storage{
		Client: client,
	}
}

func (s *storage) Set(ctx context.Context, userID string, item *film.Item) error {
	var (
		score, ok = item.Scores[userID]
		filmRef   = s.Collection(fire.FilmsCollection).Doc(item.ID)
		userRef   = s.Collection(fire.UsersCollection).Doc(userID)
	)

	err := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		userDoc, err := tx.Get(userRef)
		if err != nil {
			return errors.Wrap(err, "get user doc")
		}

		u, err := user.Parse(userDoc)
		if err != nil {
			return errors.Wrap(err, "parse user doc")
		}

		if ok {
			if len(u.Scores) == 0 {
				u.Scores = make(map[string]film.Score)
			}
			u.Scores[item.ID] = score
		} else {
			delete(u.Scores, item.ID)
		}

		if err := tx.Set(filmRef, item); err != nil {
			return errors.Wrap(err, "tx set film doc")
		}
		return errors.Wrap(tx.Set(userRef, u), "tx set user doc")
	})

	return errors.Wrap(err, "run set film transaction")
}

func (s *storage) Comments(ctx context.Context, filmID string) ([]film.Comment, error) {
	comments := make([]film.Comment, 0, 10)
	iter := s.Collection(fire.FilmsCollection).Doc(filmID).Collection(fire.CommentsCollection).Documents(ctx)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, errors.Wrap(err, "get next iterator")
		}
		c, err := film.ParseComment(doc)
		if err != nil {
			return nil, errors.Wrap(err, "parse film doc")
		}
		comments = append(comments, *c)
	}
	return comments, nil
}

func (s *storage) AddComment(ctx context.Context, filmID string, comment *film.Comment) error {
	_, _, err := s.Collection(fire.FilmsCollection).Doc(filmID).Collection(fire.CommentsCollection).Add(ctx, comment)
	if err != nil {
		return fmt.Errorf("add comment to collection: %+v", err)
	}
	return nil
}

func (s *storage) All(ctx context.Context) (film.Items, error) {
	films := make(film.Items, 0, approximateFilmsNumber)
	iter := s.Collection(fire.FilmsCollection).Documents(ctx)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, errors.Wrap(err, "get next iterator")
		}
		f, err := film.Parse(doc)
		if err != nil {
			return nil, errors.Wrap(err, "parse film doc")
		}
		films = append(films, *f)
	}
	return films, nil
}

func (s *storage) User(ctx context.Context, userID string) ([]string, error) {
	userDoc, err := s.Collection(fire.UsersCollection).Doc(userID).Get(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "get user")
	}
	u, err := user.Parse(userDoc)
	if err != nil {
		return nil, errors.Wrap(err, "parse user doc")
	}

	films := make([]string, 0, len(u.Scores))
	for k, _ := range u.Scores {
		films = append(films, k)
	}
	return films, nil
}
