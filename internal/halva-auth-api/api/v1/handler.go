package apiv1

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4/middleware"

	"github.com/labstack/echo/v4"
)

const (
	callbackPath     = "/auth/callback"
	discordAvatarURL = "https://cdn.discordapp.com/avatars/"
)

type jwtService interface {
	Generate(userID string) (string, error)
	Authorization(next echo.HandlerFunc) echo.HandlerFunc
}

type loginService interface {
	RedirectURL(redirectURL, key string) string
	GetDiscordInfo(ctx context.Context, authCode, reqState, key string) (string, string, string, error)
}

type handler struct {
	host     string
	port     string
	auth     loginService
	jwt      jwtService
	tokenTTL time.Duration
}

func New(host, port string, login loginService, jwtService jwtService, tokenTTL time.Duration) *handler {
	return &handler{
		host:     host,
		port:     port,
		auth:     login,
		jwt:      jwtService,
		tokenTTL: tokenTTL,
	}
}

func (h *handler) Run() {
	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	e.GET("/api/v1/login", h.login)
	e.GET("/api/v1/hello", h.hello, h.jwt.Authorization)
	e.GET(callbackPath, h.callback)

	e.Logger.Fatal(e.Start(":" + h.port))
}

func (h *handler) login(c echo.Context) error {
	path := h.host + ":" + h.port + callbackPath
	return c.Redirect(http.StatusTemporaryRedirect, h.auth.RedirectURL(path, c.RealIP()))
}

func (h *handler) hello(c echo.Context) error {
	return c.String(http.StatusOK, "Hello world!")
}

func (h *handler) callback(c echo.Context) error {
	userID, username, avatar, err := h.auth.GetDiscordInfo(c.Request().Context(), c.FormValue("code"), c.FormValue("state"), c.RealIP())
	if err != nil {
		return err
	}
	token, err := h.jwt.Generate(userID)
	if err != nil {
		return err
	}

	resp := loginResponse{
		Token:      token,
		Username:   username,
		Avatar:     discordAvatarURL + userID + "/" + avatar,
		Expiration: time.Now().Add(h.tokenTTL),
	}
	return c.JSON(http.StatusOK, resp)
}

type loginResponse struct {
	Token      string    `json:"token"`
	Username   string    `json:"username"`
	Avatar     string    `json:"avatar"`
	Expiration time.Time `json:"expiration"`
}
