package film

import (
	"time"

	"cloud.google.com/go/firestore"
	"github.com/pkg/errors"
)

type Score int8

const (
	BadScore       Score = -1
	NeutralScore         = 0
	GoodScore            = 1
	ExcellentScore       = 2
)

// Item TODO: user tags
type Item struct {
	ID                       string           `firestore:"-" json:"id"`
	Title                    string           `firestore:"title,omitempty" json:"title"`
	TitleOriginal            string           `firestore:"title_original,omitempty" json:"title_original,omitempty"`
	Poster                   string           `firestore:"cover,omitempty" json:"cover,omitempty"`
	Cover                    string           `firestore:"poster,omitempty" json:"poster,omitempty"`
	Director                 string           `firestore:"director,omitempty" json:"director,omitempty"`
	Description              string           `firestore:"description,omitempty" json:"description,omitempty"`
	ShortDescription         string           `firestore:"short_description,omitempty" json:"short_description,omitempty"`
	Duration                 string           `firestore:"duration,omitempty" json:"duration,omitempty"`
	Scores                   map[string]Score `firestore:"scores" json:"scores,omitempty"`
	Comments                 []Comment        `firestore:"-" json:"comments,omitempty"`
	NoComments               bool             `firestore:"-" json:"-"`
	URL                      string           `firestore:"kinopoisk,omitempty" json:"kinopoisk,omitempty"`
	RatingKinopoisk          float64          `firestore:"rating_kinopoisk,omitempty" json:"rating_kinopoisk,omitempty"`
	RatingKinopoiskVoteCount int              `firestore:"rating_kinopoisk_vote_count,omitempty" json:"rating_kinopoisk_vote_count,omitempty"`
	RatingImdb               float64          `firestore:"rating_imdb,omitempty" json:"rating_imdb,omitempty"`
	RatingImdbVoteCount      int              `firestore:"rating_imdb_vote_count,omitempty" json:"rating_imdb_vote_count,omitempty"`
	Year                     int              `firestore:"year,omitempty" json:"year,omitempty"`
	FilmLength               int              `firestore:"film_length,omitempty" json:"film_length,omitempty"`
	Serial                   bool             `firestore:"serial" json:"serial"`
	ShortFilm                bool             `firestore:"short_film" json:"short_film"`
	Genres                   []string         `firestore:"genres,omitempty" json:"genres,omitempty"`
	UpdatedAt                time.Time        `firestore:"updated_at,omitempty" json:"updated_at,omitempty"`
	CreatedAt                time.Time        `firestore:"created_at,omitempty" json:"created_at,omitempty"`
}

type Comment struct {
	UserID    string    `firestore:"user_id" json:"user_id"`
	Text      string    `firestore:"text" json:"text"`
	CreatedAt time.Time `firestore:"created_at" json:"created_at"`
}

func Parse(doc *firestore.DocumentSnapshot) (*Item, error) {
	var f Item
	if err := doc.DataTo(&f); err != nil {
		return nil, errors.Wrap(err, "unmarshall data")
	}
	f.ID = doc.Ref.ID
	return &f, nil
}

func ParseComment(doc *firestore.DocumentSnapshot) (*Comment, error) {
	var c Comment
	if err := doc.DataTo(&c); err != nil {
		return nil, errors.Wrap(err, "unmarshall data")
	}
	return &c, nil
}
