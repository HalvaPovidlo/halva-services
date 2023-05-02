package kinopoisk

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/pkg/errors"

	"github.com/HalvaPovidlo/halva-services/internal/pkg/film"
)

const (
	filmURL   = `https://www.kinopoisk.ru/film/`
	seriesURL = `https://www.kinopoisk.ru/series/`

	apiFilms      = "https://kinopoiskapiunofficial.tech/api/v2.2/films/"
	xAPIKeyHeader = "X-API-KEY"
)

type kpFilm struct {
	KinopoiskID              int       `json:"kinopoiskId"`
	ImdbID                   string    `json:"imdbId"`
	NameRu                   string    `json:"nameRu"`
	NameOriginal             string    `json:"nameOriginal"`
	PosterURL                string    `json:"posterUrl"`
	CoverURL                 string    `json:"coverUrl"`
	RatingKinopoisk          float64   `json:"ratingKinopoisk"`
	RatingKinopoiskVoteCount int       `json:"ratingKinopoiskVoteCount"`
	RatingImdb               float64   `json:"ratingImdb"`
	RatingImdbVoteCount      int       `json:"ratingImdbVoteCount"`
	Year                     int       `json:"year"`
	FilmLength               int       `json:"filmLength"`
	Description              string    `json:"description"`
	Genres                   []kpGenre `json:"genres"`
	Serial                   bool      `json:"serial"`
	ShortFilm                bool      `json:"shortFilm"`
	Completed                bool      `json:"completed"`
	WebURL                   string    `json:"webUrl"`
}

type kpGenre struct {
	Genre string `json:"genre"`
}

type kinopoisk struct {
	apiKey string
	client *http.Client
}

func New(apiKey string) *kinopoisk {
	return &kinopoisk{
		apiKey: apiKey,
		client: &http.Client{},
	}
}

func (k *kinopoisk) GetFilm(ctx context.Context, id string) (*kpFilm, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiFilms+id, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add(xAPIKeyHeader, k.apiKey)
	resp, err := k.client.Do(req)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("status not ok: " + string(data))
	}

	var film kpFilm
	if err := json.Unmarshal(data, &film); err != nil {
		return nil, errors.Wrapf(err, "unmarshall film")
	}
	return &film, nil
}

func BuildFilm(kf *kpFilm) *film.Item {
	var genres []string
	for i := range kf.Genres {
		genres = append(genres, kf.Genres[i].Genre)
	}
	return &film.Item{
		ID:                       IDFromKinopoiskURL(kf.WebURL),
		Title:                    kf.NameRu,
		TitleOriginal:            kf.NameOriginal,
		Poster:                   kf.PosterURL,
		Cover:                    kf.CoverURL,
		Description:              kf.Description,
		URL:                      kf.WebURL,
		RatingKinopoisk:          kf.RatingKinopoisk,
		RatingKinopoiskVoteCount: kf.RatingKinopoiskVoteCount,
		RatingImdb:               kf.RatingImdb,
		RatingImdbVoteCount:      kf.RatingImdbVoteCount,
		Year:                     kf.Year,
		FilmLength:               kf.FilmLength,
		Serial:                   kf.Serial,
		ShortFilm:                kf.ShortFilm,
		Genres:                   genres,
	}
}

func MergeFilm(kf *kpFilm, f *film.Item) *film.Item {
	if f.Title == "" {
		f.Title = kf.NameRu
	}
	if f.TitleOriginal == "" {
		f.TitleOriginal = kf.NameOriginal
	}
	if f.Poster == "" {
		f.Poster = kf.PosterURL
	}
	if f.Cover == "" {
		f.Cover = kf.CoverURL
	}
	if f.Description == "" {
		f.Director = kf.Description
	}
	if f.URL == "" {
		f.URL = kf.WebURL
	}
	if f.Year == 0 {
		f.Year = kf.Year
	}
	if f.FilmLength == 0 {
		f.FilmLength = kf.FilmLength
	}
	if len(f.Genres) == 0 {
		for i := range kf.Genres {
			f.Genres = append(f.Genres, kf.Genres[i].Genre)
		}
	}
	return &film.Item{
		ID:                       f.ID,
		Title:                    f.Title,
		TitleOriginal:            f.TitleOriginal,
		Poster:                   f.Poster,
		Cover:                    f.Cover,
		Director:                 f.Director,
		Description:              f.Description,
		Duration:                 f.Duration,
		UserScore:                f.UserScore,
		Scores:                   f.Scores,
		Comments:                 f.Comments,
		WithComments:             f.WithComments,
		URL:                      f.URL,
		RatingKinopoisk:          kf.RatingKinopoisk,
		RatingKinopoiskVoteCount: kf.RatingKinopoiskVoteCount,
		RatingImdb:               kf.RatingImdb,
		RatingImdbVoteCount:      kf.RatingImdbVoteCount,
		Year:                     f.Year,
		FilmLength:               f.FilmLength,
		Serial:                   kf.Serial,
		ShortFilm:                kf.ShortFilm,
		Genres:                   f.Genres,
	}
}

func IDFromKinopoiskURL(uri string) string {
	id := strings.TrimPrefix(uri, filmURL)
	id = strings.TrimPrefix(id, seriesURL)
	return strings.Trim(id, "/")
}
