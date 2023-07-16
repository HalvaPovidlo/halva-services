package song

import (
	"time"
)

type Item struct {
	ID        string    `firestore:"-" json:"id"`
	Title     string    `firestore:"title" json:"title,omitempty"`
	LastPlay  time.Time `firestore:"last_play" json:"last_play,omitempty"`
	Count     int64     `firestore:"playbacks" json:"playbacks,omitempty"`
	URL       string    `firestore:"url,omitempty" json:"url,omitempty"`
	Service   string    `firestore:"service,omitempty" json:"service,omitempty"`
	Artist    string    `firestore:"artist_name,omitempty" json:"artist_name,omitempty"`
	ArtistURL string    `firestore:"artist_url,omitempty" json:"artist_url,omitempty"`
	Artwork   string    `firestore:"artwork_url,omitempty" json:"artwork_url,omitempty"`
	Thumbnail string    `firestore:"thumbnail_url,omitempty" json:"thumbnail_url,omitempty"`

	FilePath string
}
