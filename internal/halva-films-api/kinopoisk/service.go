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
	filmURL               = `https://www.kinopoisk.ru/film/`
	seriesURL             = `https://www.kinopoisk.ru/series/`
	apiFilms              = "https://kinopoiskapiunofficial.tech/api/v2.2/films/"
	apiStaff              = "https://kinopoiskapiunofficial.tech/api/v1/staff"
	xAPIKeyHeader         = "X-API-KEY"
	DirectorProfessionKey = "DIRECTOR"
)

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

func (k *kinopoisk) GetFilm(ctx context.Context, url string) (*film.Item, error) {
	id := k.ExtractID(url)
	kf, err := k.getFilm(ctx, id)
	if err != nil {
		return nil, err
	}
	directors, err := k.getDirectors(ctx, id)
	if err != nil {
		return nil, err
	}
	f := k.buildFilm(kf)
	f.Director = directors
	return f, nil
}

func (k *kinopoisk) getFilm(ctx context.Context, id string) (*filmResp, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiFilms+id, nil)
	if err != nil {
		return nil, errors.Wrap(err, "create request get film from kp")
	}

	req.Header.Add(xAPIKeyHeader, k.apiKey)
	resp, err := k.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "do http request")
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "read body")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("status not ok: " + string(data))
	}

	var kp filmResp
	if err := json.Unmarshal(data, &kp); err != nil {
		return nil, errors.Wrap(err, "unmarshall film")
	}
	return &kp, nil
}

// https://kinopoiskapiunofficial.tech/api/v1/staff?filmId=1209850
func (k *kinopoisk) getDirectors(ctx context.Context, id string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiStaff, nil)
	if err != nil {
		return "", errors.Wrap(err, "create request get film directors from kp")
	}
	req.Header.Add(xAPIKeyHeader, k.apiKey)
	q := req.URL.Query()
	q.Add("filmId", id)
	req.URL.RawQuery = q.Encode()

	resp, err := k.client.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "do http request")
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "read body")
	}
	if resp.StatusCode != http.StatusOK {
		return "", errors.New("status not ok: " + string(data))
	}

	var staff []staffResp
	if err := json.Unmarshal(data, &staff); err != nil {
		return "", errors.Wrap(err, "unmarshall directors")
	}

	directors := make([]string, 0, len(staff))
	for i := range staff {
		if staff[i].ProfessionKey == DirectorProfessionKey {
			if staff[i].NameRu != "" {
				directors = append(directors, staff[i].NameRu)
			} else {
				directors = append(directors, staff[i].NameEn)
			}
		}
	}

	return strings.Join(directors, ", "), nil
}

func (k *kinopoisk) buildFilm(kf *filmResp) *film.Item {
	var genres []string
	for i := range kf.Genres {
		genres = append(genres, kf.Genres[i].Genre)
	}
	return &film.Item{
		ID:                       k.ExtractID(kf.WebURL),
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

func MergeFilm(kf *filmResp, f *film.Item) *film.Item {
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
		f.Description = kf.Description
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

func (k *kinopoisk) ExtractID(uri string) string {
	id := strings.TrimPrefix(uri, filmURL)
	id = strings.TrimPrefix(id, seriesURL)
	id = strings.TrimSuffix(id, "\n")
	id = strings.Split(id, "?")[0]
	id = strings.Split(id, "&")[0]
	id = strings.Split(id, "#")[0]
	return strings.Split(id, "/")[0]
}

type filmResp struct {
	KinopoiskID              int     `json:"kinopoiskId"`
	ImdbID                   string  `json:"imdbId"`
	NameRu                   string  `json:"nameRu"`
	NameOriginal             string  `json:"nameOriginal"`
	PosterURL                string  `json:"posterUrl"`
	CoverURL                 string  `json:"coverUrl"`
	RatingKinopoisk          float64 `json:"ratingKinopoisk"`
	RatingKinopoiskVoteCount int     `json:"ratingKinopoiskVoteCount"`
	RatingImdb               float64 `json:"ratingImdb"`
	RatingImdbVoteCount      int     `json:"ratingImdbVoteCount"`
	Year                     int     `json:"year"`
	FilmLength               int     `json:"filmLength"`
	Description              string  `json:"description"`
	Genres                   []genre `json:"genres"`
	Serial                   bool    `json:"serial"`
	ShortFilm                bool    `json:"shortFilm"`
	Completed                bool    `json:"completed"`
	WebURL                   string  `json:"webUrl"`
}

type genre struct {
	Genre string `json:"genre"`
}

type staffResp struct {
	StaffId        int     `json:"staffId"`
	NameRu         string  `json:"nameRu"`
	NameEn         string  `json:"nameEn"`
	Description    *string `json:"description"`
	PosterUrl      string  `json:"posterUrl"`
	ProfessionText string  `json:"professionText"`
	ProfessionKey  string  `json:"professionKey"`
}
