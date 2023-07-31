package apiv1

import (
	"context"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	films "github.com/HalvaPovidlo/halva-services/internal/halva-films-api/film"
	pfilm "github.com/HalvaPovidlo/halva-services/internal/pkg/film"
)

const (
	SortLexicographic     = "lexicographic"
	SortRatingKinopoisk   = "kinopoisk"
	SortRatingImdb        = "imdb"
	SortRatingHalva       = "halva"
	SortRatingSum         = "sum"
	SortRatingAverage     = "average"
	SortRatingScoreNumber = "score_number"
	SortCreatedAt         = "created"
	SortUpdatedAt         = "updated"

	errEmptyID      = "empty id"
	errFilmNotFound = "film not found"
)

type filmService interface {
	New(ctx context.Context, userID, url string, score pfilm.Score) (*pfilm.Item, error)
	Get(ctx context.Context, url string) (*pfilm.Item, error)
	All(ctx context.Context) (pfilm.Items, error)
	Score(ctx context.Context, userID, url string, score pfilm.Score) (*pfilm.Item, error)
	RemoveScore(ctx context.Context, userID, url string) (*pfilm.Item, error)
	User(ctx context.Context, userID string) (pfilm.Items, error)
	Comment(ctx context.Context, userID, url, text string) (*pfilm.Item, error)
}

type jwtService interface {
	Authorization(next echo.HandlerFunc) echo.HandlerFunc
	ExtractUserID(c echo.Context) (string, error)
}

type handler struct {
	film        filmService
	jwt         jwtService
	defaultSort string
	tokenTTL    time.Duration
}

func New(filmService filmService, jwtService jwtService, defaultSort string) *handler {
	return &handler{
		jwt:         jwtService,
		film:        filmService,
		defaultSort: defaultSort,
	}
}

func (h *handler) RegisterRoutes(e *echo.Echo) {
	e.GET("/api/v1/public/films/:id/get", h.get)
	e.GET("/api/v1/public/films/all", h.all)

	e.POST("/api/v1/films/new", h.new, h.jwt.Authorization)
	e.GET("/api/v1/films/:id/get", h.get, h.jwt.Authorization)
	e.GET("/api/v1/films/all", h.all, h.jwt.Authorization)
	e.GET("/api/v1/films/my", h.my, h.jwt.Authorization)
	e.PATCH("/api/v1/films/:id/score", h.score, h.jwt.Authorization)
	e.PATCH("/api/v1/films/:id/unscore", h.removeScore, h.jwt.Authorization)
	e.POST("/api/v1/films/:id/comment", h.comment, h.jwt.Authorization)
}

func (h *handler) new(c echo.Context) error {
	url := c.QueryParam("url")
	scoreStr := c.QueryParam("score")
	if url == "" || scoreStr == "" {
		return c.String(http.StatusBadRequest, "url or score param is empty")
	}

	userID, err := h.jwt.ExtractUserID(c)
	if err != nil {
		return c.String(http.StatusUnauthorized, err.Error())
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

	return c.JSON(http.StatusOK, build(film, userID, false))
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
		return c.String(http.StatusNotFound, errFilmNotFound)
	case err != nil:
		return err
	}

	return c.JSON(http.StatusOK, build(film, userID, true))
}

func (h *handler) my(c echo.Context) error {
	userID, err := h.jwt.ExtractUserID(c)
	if err != nil {
		return c.String(http.StatusUnauthorized, err.Error())
	}
	userFilms, err := h.film.User(c.Request().Context(), userID)
	switch {
	case errors.Is(err, films.ErrNotFound):
		return c.String(http.StatusNotFound, "user not found")
	case err != nil:
		return err
	}

	h.sortFilms(userFilms, h.defaultSort)
	return c.JSON(http.StatusOK, buildAll(userFilms, userID))
}

func (h *handler) all(c echo.Context) error {
	userID, _ := h.jwt.ExtractUserID(c)
	sort := c.QueryParam("sort")

	allFilms, err := h.film.All(c.Request().Context())
	if err != nil {
		return err
	}

	h.sortFilms(allFilms, sort)

	return c.JSON(http.StatusOK, buildAll(allFilms, userID))
}

func (h *handler) score(c echo.Context) error {
	id := c.Param("id")
	scoreStr := c.QueryParam("score")
	if id == "" || scoreStr == "" {
		return c.String(http.StatusBadRequest, "url or score param is empty")
	}

	userID, err := h.jwt.ExtractUserID(c)
	if err != nil {
		return c.String(http.StatusUnauthorized, err.Error())
	}

	score, err := strconv.Atoi(scoreStr)
	if err != nil || score < -1 || score > 2 {
		return c.String(http.StatusBadRequest, "score should be in (-1, 0, 1, 2)")
	}

	film, err := h.film.Score(c.Request().Context(), userID, id, pfilm.Score(score))
	switch {
	case errors.Is(err, films.ErrNotFound):
		return c.String(http.StatusNotFound, errFilmNotFound)
	case err != nil:
		return err
	}

	return c.JSON(http.StatusOK, build(film, userID, false))
}

func (h *handler) removeScore(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.String(http.StatusBadRequest, errEmptyID)
	}

	userID, err := h.jwt.ExtractUserID(c)
	if err != nil {
		return c.String(http.StatusUnauthorized, err.Error())
	}

	film, err := h.film.RemoveScore(c.Request().Context(), userID, id)
	switch {
	case errors.Is(err, films.ErrNotFound):
		return c.String(http.StatusNotFound, errFilmNotFound)
	case errors.Is(err, films.ErrNoScore):
		return c.String(http.StatusNotFound, "film has no score from you")
	case err != nil:
		return err
	}

	return c.JSON(http.StatusOK, build(film, "", false))
}

func (h *handler) comment(c echo.Context) error {
	userID, err := h.jwt.ExtractUserID(c)
	if err != nil {
		return c.String(http.StatusUnauthorized, err.Error())
	}

	id := c.Param("id")
	if id == "" {
		return c.String(http.StatusBadRequest, errEmptyID)
	}

	var req commentRequest
	if err := (&echo.DefaultBinder{}).BindBody(c, &req); err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}

	film, err := h.film.Comment(c.Request().Context(), userID, id, req.Text)
	switch {
	case errors.Is(err, films.ErrNotFound):
		return c.String(http.StatusNotFound, errFilmNotFound)
	case err != nil:
		return err
	}

	return c.JSON(http.StatusOK, build(film, userID, true))
}

func (h *handler) sortFilms(films pfilm.Items, sort string) {
	switch sort {
	case SortLexicographic:
		films.SortLexicographic()
	case SortRatingKinopoisk:
		films.SortKinopoisk()
	case SortRatingImdb:
		films.SortIMDB()
	case SortRatingHalva:
		films.SortHalva()
	case SortRatingSum:
		films.SortSum()
	case SortRatingAverage:
		films.SortAverage()
	case SortRatingScoreNumber:
		films.SortScoreNumber()
	case SortUpdatedAt:
		films.SortUpdatedAt()
	case SortCreatedAt:
		films.SortCreatedAt()
	default:
		if sort == h.defaultSort {
			return
		}
		h.sortFilms(films, h.defaultSort)
	}
}

func build(film *pfilm.Item, userID string, withComments bool) *filmResponse {
	var score *int
	if v, ok := film.Scores[userID]; userID != "" && ok {
		vint := int(v)
		score = &vint
	}

	var scores map[string]int
	if userID != "" {
		scores = make(map[string]int, len(film.Scores))
		for k, v := range film.Scores {
			scores[k] = int(v)
		}
	}

	var comments []commentResp
	if userID != "" && withComments && !film.NoComments {
		comments = make([]commentResp, 0, len(film.Comments))
		for i := range film.Comments {
			comments = append(comments, commentResp{
				UserID:    film.Comments[i].UserID,
				Text:      film.Comments[i].Text,
				CreatedAt: film.Comments[i].CreatedAt,
			})
		}
		sort.Slice(comments, func(i, j int) bool {
			return comments[i].CreatedAt.Before(comments[j].CreatedAt)
		})
	}

	return &filmResponse{
		ID:               film.ID,
		Title:            film.Title,
		TitleOriginal:    film.TitleOriginal,
		Poster:           film.Poster,
		Cover:            film.Cover,
		Director:         film.Director,
		Description:      film.Description,
		ShortDescription: film.ShortDescription,
		Duration:         film.Duration,
		UserScore:        score,
		Scores:           scores,
		URL:              film.URL,
		RatingKinopoisk:  film.RatingKinopoisk,
		RatingImdb:       film.RatingImdb,
		RatingHalva:      float64(film.Halva()),
		RatingSum:        float64(film.Sum()),
		RatingAverage:    float64(film.Average()),
		Year:             film.Year,
		FilmLength:       film.FilmLength,
		Serial:           film.Serial,
		ShortFilm:        film.ShortFilm,
		Genres:           film.Genres,
		Comments:         comments,
		UpdatedAt:        film.UpdatedAt,
		CreatedAt:        film.CreatedAt,
	}
}

func buildAll(all pfilm.Items, userID string) allFilmsResponse {
	var resp allFilmsResponse
	resp.Films = make([]filmResponse, 0, len(all))
	for i := range all {
		resp.Films = append(resp.Films, *build(&all[i], userID, false))
	}
	return resp
}

type filmResponse struct {
	ID               string         `json:"id"`
	Title            string         `json:"title"`
	TitleOriginal    string         `json:"title_original,omitempty"`
	Poster           string         `json:"cover,omitempty"`
	Cover            string         `json:"poster,omitempty"`
	Director         string         `json:"director,omitempty"`
	Description      string         `json:"description,omitempty"`
	ShortDescription string         `json:"short_description,omitempty"`
	Duration         string         `json:"duration,omitempty"`
	UserScore        *int           `json:"user_score,omitempty"`
	Scores           map[string]int `json:"scores,omitempty"`
	URL              string         `json:"kinopoisk,omitempty"`
	RatingKinopoisk  float64        `json:"rating_kinopoisk"`
	RatingImdb       float64        `json:"rating_imdb"`
	RatingHalva      float64        `json:"rating_halva"`
	RatingSum        float64        `json:"rating_sum"`
	RatingAverage    float64        `json:"rating_average"`
	Year             int            `json:"year,omitempty"`
	FilmLength       int            `json:"film_length,omitempty"`
	Serial           bool           `json:"serial"`
	ShortFilm        bool           `json:"short_film"`
	Genres           []string       `json:"genres,omitempty"`
	Comments         []commentResp  `json:"comments,omitempty"`
	UpdatedAt        time.Time      `json:"updated_at,omitempty"`
	CreatedAt        time.Time      `json:"created_at,omitempty"`
}

type commentResp struct {
	UserID    string    `json:"user_id"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

type allFilmsResponse struct {
	Films []filmResponse `json:"films"`
}

type commentRequest struct {
	Text string `json:"text"`
}
