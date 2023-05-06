package user

import "github.com/HalvaPovidlo/halva-services/internal/pkg/film"

type Items []Item

type Item struct {
	ID       string                `firestore:"-" json:"id"`
	Username string                `firestore:"username" json:"username,omitempty"`
	Avatar   string                `firestore:"avatar,omitempty" json:"avatar,omitempty"`
	Scores   map[string]film.Score `firestore:"scores" json:"scores,omitempty"`
}
