package film

import (
	"context"

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

func (s *storage) SetFilm(ctx context.Context, userID string, item *film.Item) error {
	score, ok := item.Scores[userID]

	filmRef := s.Collection(fire.FilmsCollection).Doc(item.ID)
	userRef := s.Collection(fire.UsersCollection).Doc(userID)
	err := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		userDoc, err := tx.Get(userRef)
		if err != nil {
			return errors.Wrap(err, "get user doc")
		}

		var u user.Item
		if err := userDoc.DataTo(&u); err != nil {
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

func (s *storage) AllFilms(ctx context.Context) (film.Items, error) {
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
		f, err := parseFilm(doc)
		if err != nil {
			return nil, err
		}
		films = append(films, *f)
	}
	return films, nil
}

func parseFilm(doc *firestore.DocumentSnapshot) (*film.Item, error) {
	var f film.Item
	if err := doc.DataTo(&f); err != nil {
		return nil, errors.Wrap(err, "parse film doc")
	}
	f.ID = doc.Ref.ID
	return &f, nil
}
