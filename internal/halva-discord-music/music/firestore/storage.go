package firestore

import (
	"context"
	"fmt"
	"google.golang.org/api/iterator"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	psong "github.com/HalvaPovidlo/halva-services/internal/pkg/song"
	fire "github.com/HalvaPovidlo/halva-services/pkg/firestore"
)

const approximateSongsNumber = 512

type storage struct {
	*firestore.Client
}

func NewStorage(client *firestore.Client) *storage {
	return &storage{
		Client: client,
	}
}

func (s *storage) Get(ctx context.Context, id psong.IDType) (*psong.Item, error) {
	doc, err := s.Collection(fire.SongsCollection).Doc(string(id)).Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("get song doc: %+w", err)
	}

	item, err := psong.Parse(doc)
	if err != nil {
		return nil, fmt.Errorf("parse song doc: %+w", err)
	}

	return item, nil
}

func (s *storage) Set(ctx context.Context, userID string, item *psong.Item) error {
	var (
		songRef     = s.Collection(fire.SongsCollection).Doc(string(item.ID))
		userSongRef = s.Collection(fire.UsersCollection).Doc(userID).Collection(fire.SongsCollection).Doc(string(item.ID))
	)

	err := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		userCount := int64(0)
		userSongDoc, err := tx.Get(userSongRef)
		if err != nil && status.Code(err) != codes.NotFound {
			return fmt.Errorf("get user song doc: %+w", err)
		}

		us, err := psong.Parse(userSongDoc)
		if err != nil {
			return fmt.Errorf("parse user song doc: %+w", err)
		}

		userCount = us.Count
		userCount++

		if err := tx.Set(songRef, item); err != nil {
			return fmt.Errorf("tx set song doc: %+w", err)
		}
		userSong := item
		userSong.Count = userCount

		if err := tx.Set(userSongRef, userSong); err != nil {
			return fmt.Errorf("tx set user song doc: %+w", err)
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("run set song transaction: %+w", err)
	}
	return nil
}

func (s *storage) All(ctx context.Context) ([]psong.Item, error) {
	songs := make([]psong.Item, 0)
	iter := s.Collection(fire.SongsCollection).Documents(ctx)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("get next iterator: %+w", err)
		}
		song, err := psong.Parse(doc)
		if err != nil {
			return nil, fmt.Errorf("parse song doc: %+w", err)
		}
		songs = append(songs, *song)
	}
	return songs, nil
}
