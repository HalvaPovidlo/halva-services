package search

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"

	"github.com/HalvaPovidlo/halva-services/internal/pkg/song"
)

const (
	videoPrefix   = "https://youtube.com/watch?v="
	channelPrefix = "https://youtube.com/channel/"
	videoKind     = "youtube#video"
	videoType     = "audio/mp4"
	maxResult     = 10
)

type youtubeService struct {
	client *youtube.Service
}

func newYouTubeSearcher(ctx context.Context, credentials string) (*youtubeService, error) {
	client, err := youtube.NewService(ctx, option.WithCredentialsFile(credentials))
	if err != nil {
		return nil, fmt.Errorf("init youtube client: %+w", err)
	}
	return &youtubeService{
		client: client,
	}, nil
}

func (y *youtubeService) search(ctx context.Context, query string) (*song.Item, error) {
	response, err := y.client.Search.List([]string{"id, snippet"}).Q(query).MaxResults(maxResult).Context(ctx).Do()
	if err != nil || len(response.Items) == 0 {
		return nil, ErrSongNotFound
	}

	for _, resp := range response.Items {
		if resp.Id.Kind == videoKind {
			art, thumb := getImages(resp.Snippet.Thumbnails)
			return &song.Item{
				ID:        song.ID(resp.Id.VideoId, song.ServiceYoutube),
				Title:     resp.Snippet.Title,
				LastPlay:  time.Now(),
				Count:     1,
				URL:       videoPrefix + resp.Id.VideoId,
				Service:   string(song.ServiceYoutube),
				Artist:    resp.Snippet.ChannelTitle,
				ArtistURL: channelPrefix + resp.Snippet.ChannelId,
				Artwork:   art,
				Thumbnail: thumb,
			}, nil
		}
	}

	return nil, ErrSongNotFound
}

func getImages(details *youtube.ThumbnailDetails) (artwork, thumbnail string) {
	if details == nil {
		return
	}

	switch {
	case details.Maxres != nil:
		artwork = details.Maxres.Url
	case details.High != nil:
		artwork = details.High.Url
	case details.Medium != nil:
		artwork = details.Medium.Url
	case details.Standard != nil:
		artwork = details.Standard.Url
	case details.Default != nil:
		artwork = details.Default.Url
	}

	switch {
	case details.Standard != nil:
		thumbnail = details.Standard.Url
	case details.Default != nil:
		thumbnail = details.Default.Url
	}

	return
}
