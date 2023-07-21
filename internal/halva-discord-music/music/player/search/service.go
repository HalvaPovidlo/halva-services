package search

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	psong "github.com/HalvaPovidlo/halva-services/internal/pkg/song"
)

var (
	ErrSongNotFound   = fmt.Errorf("song not found")
	ErrServiceUnknown = fmt.Errorf("service unknown")
)

type storage interface {
	Get(ctx context.Context, id psong.IDType) (*psong.Item, error)
	GetAny(ctx context.Context) (*psong.Item, error)
	Add(ctx context.Context, song *psong.Item) error
}

type Request struct {
	Text    string
	Service psong.ServiceType
}

type service struct {
	youtube *youtubeService
	storage storage
}

func New(ctx context.Context, credentials string, storage storage) (*service, error) {
	youtube, err := newYouTubeSearcher(ctx, credentials)
	if err != nil {
		return nil, err
	}
	return &service{
		youtube: youtube,
		storage: storage,
	}, nil
}

func (s *service) Search(ctx context.Context, request *Request) (*psong.Item, error) {
	switch request.Service {
	case psong.ServiceYoutube:
		return s.searchYoutube(ctx, request)
	case psong.ServiceVK:
		return nil, ErrServiceUnknown
	default:
		return nil, ErrServiceUnknown
	}
}

func (s *service) searchYoutube(ctx context.Context, request *Request) (*psong.Item, error) {
	if id := extractYoutubeID(request.Text); id != "" {
		song, err := s.storage.Get(ctx, psong.ID(id, psong.ServiceYoutube))
		switch {
		case err == nil:
			song.Count++
			song.LastPlay = time.Now()
			if err := s.storage.Add(ctx, song); err != nil {
				return nil, fmt.Errorf("add song to storage: %+w", err)
			}
			return song, nil
		case status.Code(err) == codes.NotFound:
			// pass
		case err != nil:
			return nil, fmt.Errorf("get song from storage: %+w", err)
		}
	}

	song, err := s.youtube.search(ctx, request.Text)
	if err != nil {
		return nil, fmt.Errorf("search song on youtube: %+w", err)
	}

	if err := s.storage.Add(ctx, song); err != nil {
		return nil, fmt.Errorf("add song to storage: %+w", err)
	}

	return song, nil
}

func (s *service) Radio(ctx context.Context) (*psong.Item, error) {
	song, err := s.storage.GetAny(ctx)
	if err != nil {
		return nil, fmt.Errorf("get any song from storage: %+w", err)
	}
	return song, err
}

func extractYoutubeID(url string) string {
	url = strings.TrimPrefix(url, `https:`)
	url = strings.TrimPrefix(url, `http:`)
	url = strings.TrimPrefix(url, `//`)
	url = strings.TrimPrefix(url, `www.`)
	url = strings.TrimPrefix(url, `m.`)
	url = strings.TrimPrefix(url, `music.`)
	url = strings.TrimPrefix(url, `youtu.be/`)
	url = strings.TrimPrefix(url, `youtube.com/`)
	url = strings.TrimPrefix(url, `youtube-nocookie.com/`)
	url = strings.TrimPrefix(url, `embed/`)
	url = strings.TrimPrefix(url, `shorts/`)
	url = strings.TrimPrefix(url, `v/`)
	url = strings.TrimPrefix(url, `live/`)
	url = strings.TrimPrefix(url, `watch?`)
	url = strings.TrimPrefix(url, `v=`)
	url = strings.TrimPrefix(url, `e/`)
	url = strings.TrimPrefix(url, `feature=player_embedded&v=`)
	url = strings.TrimPrefix(url, `app=desktop&v=`)
	url = strings.TrimPrefix(url, `attribution_link?a=`)

	url = strings.TrimSuffix(url, "\n")
	url = strings.Split(url, "?")[0]
	url = strings.Split(url, "&")[0]
	url = strings.Split(url, "#")[0]

	match, _ := regexp.Match("^[-_a-zA-Z0-9]+$", []byte(url))
	if !match {
		return ""
	}

	return url
}
