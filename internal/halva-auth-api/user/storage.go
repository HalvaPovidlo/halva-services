package user

import (
	"context"

	"cloud.google.com/go/firestore"
	"github.com/pkg/errors"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/HalvaPovidlo/halva-services/internal/pkg/user"
	fire "github.com/HalvaPovidlo/halva-services/pkg/firestore"
)

const approximateUsersNumber = 10

type storage struct {
	*firestore.Client
}

func NewStorage(client *firestore.Client) *storage {
	return &storage{
		Client: client,
	}
}

func (s *storage) Upsert(ctx context.Context, new *user.Item) error {
	userRef := s.Collection(fire.UsersCollection).Doc(new.ID)
	err := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		userDoc, err := tx.Get(userRef)
		switch {
		case status.Code(err) == codes.NotFound:
			return errors.Wrap(tx.Set(userRef, new), "tx set user doc")
		case err != nil:
			return errors.Wrap(err, "get user doc")
		}

		old, err := parseUser(userDoc)
		if err != nil {
			return err
		}
		old.Username = new.Username
		old.Avatar = new.Avatar

		return errors.Wrap(tx.Set(userRef, old), "tx set user doc")
	})

	return errors.Wrap(err, "run upsert user transaction")
}

func (s *storage) All(ctx context.Context) (user.Items, error) {
	users := make(user.Items, 0, approximateUsersNumber)
	iter := s.Collection(fire.UsersCollection).Documents(ctx)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, errors.Wrap(err, "get next iterator")
		}
		u, err := parseUser(doc)
		if err != nil {
			return nil, err
		}
		users = append(users, *u)
	}
	return users, nil
}

func parseUser(doc *firestore.DocumentSnapshot) (*user.Item, error) {
	var u user.Item
	if err := doc.DataTo(&u); err != nil {
		return nil, errors.Wrap(err, "parse user doc")
	}
	u.ID = doc.Ref.ID
	return &u, nil
}
