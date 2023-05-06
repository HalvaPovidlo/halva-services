package user

import (
	"cloud.google.com/go/firestore"
	"github.com/pkg/errors"

	"github.com/HalvaPovidlo/halva-services/internal/pkg/film"
)

type Items []Item

type Item struct {
	ID       string                `firestore:"-" json:"id"`
	Username string                `firestore:"username" json:"username,omitempty"`
	Avatar   string                `firestore:"avatar,omitempty" json:"avatar,omitempty"`
	Scores   map[string]film.Score `firestore:"scores" json:"scores,omitempty"`
}

func Parse(doc *firestore.DocumentSnapshot) (*Item, error) {
	var u Item
	if err := doc.DataTo(&u); err != nil {
		return nil, errors.Wrap(err, "unmarshall data")
	}
	u.ID = doc.Ref.ID
	return &u, nil
}
