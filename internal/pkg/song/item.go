package song

import (
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
)

type ServiceType string

const (
	ServiceYoutube ServiceType = "youtube"
	ServiceVK      ServiceType = "vk"
)

type IDType string

type Item struct {
	ID        IDType    `firestore:"-" json:"id"`
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

func ID(songID string, service ServiceType) IDType {
	return IDType(string(service) + "_" + songID)
}

func Parse(doc *firestore.DocumentSnapshot) (*Item, error) {
	var s Item
	if err := doc.DataTo(&s); err != nil {
		if err != nil {
			var old *oldSong
			err = doc.DataTo(&old)
			if err != nil {
				return nil, fmt.Errorf("unmarshall data: %+w", err)
			}
			s = buildNewSong(old)
		}
	}
	s.ID = IDType(doc.Ref.ID)
	return &s, nil
}

func buildNewSong(s *oldSong) Item {
	return Item{
		Title:     s.Title,
		LastPlay:  s.LastPlay.Time,
		Count:     int64(s.Playbacks),
		URL:       s.URL,
		Service:   s.Service,
		Artist:    s.ArtistName,
		ArtistURL: s.ArtistURL,
		Artwork:   s.ArtworkURL,
		Thumbnail: s.ThumbnailURL,
	}
}

type playDate struct {
	time.Time
}

type oldSong struct {
	Title        string   `firestore:"title,omitempty" csv:"title" json:"title,omitempty"`
	URL          string   `firestore:"url,omitempty" csv:"url,omitempty" json:"url,omitempty"`
	Service      string   `firestore:"service,omitempty" csv:"service,omitempty" json:"service,omitempty"`
	ArtistName   string   `firestore:"artist_name,omitempty" csv:"artist_name,omitempty" json:"artist_name,omitempty"`
	ArtistURL    string   `firestore:"artist_url,omitempty" csv:"artist_url,omitempty" json:"artist_url,omitempty"`
	ArtworkURL   string   `firestore:"artwork_url,omitempty" csv:"artwork_url,omitempty" json:"artwork_url,omitempty"`
	ThumbnailURL string   `firestore:"thumbnail_url,omitempty" csv:"thumbnail_url,omitempty" json:"thumbnail_url,omitempty"`
	Playbacks    int      `firestore:"playbacks,omitempty" csv:"playbacks" json:"playbacks,omitempty"`
	LastPlay     playDate `firestore:"last_play,omitempty" csv:"last_play,omitempty" json:"last_play,omitempty"`
}
