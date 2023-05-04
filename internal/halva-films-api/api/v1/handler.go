package apiv1

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/pkg/errors"

	films "github.com/HalvaPovidlo/halva-services/internal/halva-films-api/film"
	pfilm "github.com/HalvaPovidlo/halva-services/internal/pkg/film"
)

type filmService interface {
	New(ctx context.Context, userID, url string, score pfilm.Score) (*pfilm.Item, error)
	Get(ctx context.Context, url string) (*pfilm.Item, error)
	All(ctx context.Context) (pfilm.Items, error)
	Score(ctx context.Context, userID, url string, score pfilm.Score) (*pfilm.Item, error)
	RemoveScore(ctx context.Context, userID, url string) (*pfilm.Item, error)
}

type jwtService interface {
	Authorization(next echo.HandlerFunc) echo.HandlerFunc
	ExtractUserID(c echo.Context) (string, error)
}

type handler struct {
	film     filmService
	jwt      jwtService
	tokenTTL time.Duration
}

func New(filmService filmService, jwtService jwtService) *handler {
	return &handler{
		jwt:  jwtService,
		film: filmService,
	}
}

func (h *handler) Run(port string) {
	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	e.GET("/api/v1/public/films/get", h.get)
	e.GET("/api/v1/public/films/all", h.all)

	e.POST("/api/v1/films/new", h.new, h.jwt.Authorization)
	e.GET("/api/v1/films/:id/get", h.get, h.jwt.Authorization)
	e.GET("/api/v1/films/all", h.all, h.jwt.Authorization)
	e.PATCH("/api/v1/films/:id/score", h.score, h.jwt.Authorization)
	e.PATCH("/api/v1/films/:id/unscore", h.removeScore, h.jwt.Authorization)

	e.Logger.Fatal(e.Start(":" + port))
}

func (h *handler) new(c echo.Context) error {
	url := c.QueryParam("url")
	scoreStr := c.QueryParam("score")
	if url == "" || scoreStr == "" {
		return c.String(http.StatusBadRequest, "url or score param is empty")
	}

	userID, err := h.jwt.ExtractUserID(c)
	if err != nil {
		return err
	}

	score, err := strconv.Atoi(scoreStr)
	if err != nil || score < -1 || score > 2 {
		return c.String(http.StatusBadRequest, "score should be in (-1, 0, 1, 2)")
	}

	film, err := h.film.New(c.Request().Context(), userID, url, pfilm.Score(score))
	switch {
	case errors.Is(err, films.ErrAlreadyExists):
		return c.String(http.StatusBadRequest, "Film already exists")
	case err != nil:
		return err
	}

	return c.JSON(http.StatusOK, build(film, &score))
}

func (h *handler) get(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.String(http.StatusBadRequest, "id param is empty")
	}

	userID, _ := h.jwt.ExtractUserID(c)

	film, err := h.film.Get(c.Request().Context(), id)
	switch {
	case errors.Is(err, films.ErrNotFound):
		return c.String(http.StatusNotFound, "Film not found")
	case err != nil:
		return err
	}
	var score *int
	if v, ok := film.Scores[userID]; userID != "" && ok {
		vint := int(v)
		score = &vint
	}

	return c.JSON(http.StatusOK, build(film, score))
}

const (
	SortRatingKinopoisk = "kinopoisk"
	SortRatingImdb      = "imdb"
	SortRatingHalva     = "halva"
	SortRatingSum       = "sum"
	SortRatingAverage   = "average"
)

func (h *handler) all(c echo.Context) error {
	userID, _ := h.jwt.ExtractUserID(c)
	sort := c.QueryParam("sort")

	allFilms, err := h.film.All(c.Request().Context())
	if err != nil {
		return err
	}

	switch sort {
	case SortRatingKinopoisk:
		allFilms.SortKinopoisk()
	case SortRatingImdb:
		allFilms.SortIMDB()
	case SortRatingHalva:
		allFilms.SortHalva()
	case SortRatingSum:
		allFilms.SortSum()
	case SortRatingAverage:
		allFilms.SortAverage()
	}

	var resp AllFilmsResponse
	resp.Films = make([]filmResponse, 0, len(allFilms))

	for i := range allFilms {
		var score *int
		film := &allFilms[i]
		if userID != "" {
			if v, ok := film.Scores[userID]; ok {
				vint := int(v)
				score = &vint
			}
		}
		resp.Films = append(resp.Films, *build(film, score))
	}

	return c.JSON(http.StatusOK, resp)
}

func (h *handler) score(c echo.Context) error {
	id := c.Param("id")
	scoreStr := c.QueryParam("score")
	if id == "" || scoreStr == "" {
		return c.String(http.StatusBadRequest, "url or score param is empty")
	}

	userID, err := h.jwt.ExtractUserID(c)
	if err != nil {
		return err
	}

	score, err := strconv.Atoi(scoreStr)
	if err != nil || score < -1 || score > 2 {
		return c.String(http.StatusBadRequest, "score should be in (-1, 0, 1, 2)")
	}

	film, err := h.film.Score(c.Request().Context(), userID, id, pfilm.Score(score))
	switch {
	case errors.Is(err, films.ErrNotFound):
		return c.String(http.StatusNotFound, "Film not found")
	case err != nil:
		return err
	}

	return c.JSON(http.StatusOK, build(film, &score))
}

func (h *handler) removeScore(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.String(http.StatusBadRequest, "url or score param is empty")
	}

	userID, err := h.jwt.ExtractUserID(c)
	if err != nil {
		return err
	}

	film, err := h.film.RemoveScore(c.Request().Context(), userID, id)
	switch {
	case errors.Is(err, films.ErrNotFound):
		return c.String(http.StatusNotFound, "Film not found")
	case err != nil:
		return err
	}

	return c.JSON(http.StatusOK, build(film, nil))
}

func build(film *pfilm.Item, userScore *int) *filmResponse {
	scores := make(map[string]int, len(film.Scores))
	for k, v := range film.Scores {
		scores[k] = int(v)
	}

	return &filmResponse{
		ID:              film.ID,
		Title:           film.Title,
		TitleOriginal:   film.TitleOriginal,
		Poster:          film.Poster,
		Cover:           film.Cover,
		Director:        film.Director,
		Description:     film.Description,
		Duration:        film.Duration,
		UserScore:       userScore,
		Scores:          scores,
		URL:             film.URL,
		RatingKinopoisk: film.RatingKinopoisk,
		RatingImdb:      film.RatingImdb,
		RatingHalva:     float64(film.Halva()),
		RatingSum:       float64(film.Sum()),
		RatingAverage:   float64(film.Average()),
		Year:            film.Year,
		FilmLength:      film.FilmLength,
		Serial:          film.Serial,
		ShortFilm:       film.ShortFilm,
		Genres:          film.Genres,
	}
}

type filmResponse struct {
	ID              string         `json:"id"`
	Title           string         `json:"title"`
	TitleOriginal   string         `json:"title_original,omitempty"`
	Poster          string         `json:"cover,omitempty"`
	Cover           string         `json:"poster,omitempty"`
	Director        string         `json:"director,omitempty"`
	Description     string         `json:"description,omitempty"`
	Duration        string         `json:"duration,omitempty"`
	UserScore       *int           `json:"user_score,omitempty"`
	Scores          map[string]int `json:"scores"`
	URL             string         `json:"kinopoisk,omitempty"`
	RatingKinopoisk float64        `json:"rating_kinopoisk"`
	RatingImdb      float64        `json:"rating_imdb"`
	RatingHalva     float64        `json:"rating_halva"`
	RatingSum       float64        `json:"rating_sum"`
	RatingAverage   float64        `json:"rating_average"`
	Year            int            `json:"year,omitempty"`
	FilmLength      int            `json:"film_length,omitempty"`
	Serial          bool           `json:"serial"`
	ShortFilm       bool           `json:"short_film"`
	Genres          []string       `json:"genres,omitempty"`
}

type AllFilmsResponse struct {
	Films []filmResponse `json:"films"`
}
