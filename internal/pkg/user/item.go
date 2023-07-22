package user

import (
	"fmt"

	"cloud.google.com/go/firestore"

	"github.com/HalvaPovidlo/halva-services/internal/pkg/film"
	"github.com/HalvaPovidlo/halva-services/internal/pkg/song"
)

type Items []Item

type Item struct {
	ID       string                `firestore:"-" json:"id"`
	Username string                `firestore:"username" json:"username,omitempty"`
	Avatar   string                `firestore:"avatar,omitempty" json:"avatar,omitempty"`
	Scores   map[string]film.Score `firestore:"scores" json:"scores,omitempty"`
	Songs    map[string]song.Item  `firestore:"-" json:"songs,omitempty"`
}

func Parse(doc *firestore.DocumentSnapshot) (*Item, error) {
	var u Item
	if err := doc.DataTo(&u); err != nil {
		return nil, fmt.Errorf("unmarshall data: %+w", err)
	}
	u.ID = doc.Ref.ID
	return &u, nil
}
