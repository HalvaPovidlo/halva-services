package apiv1

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/HalvaPovidlo/halva-services/internal/halva-auth-api/auth"
	"github.com/HalvaPovidlo/halva-services/internal/pkg/user"
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

type userService interface {
	Upsert(ctx context.Context, id, username, avatar string) error
	All(ctx context.Context) (user.Items, error)
}

type handler struct {
	host     string
	port     string
	auth     loginService
	jwt      jwtService
	user     userService
	tokenTTL time.Duration
}

func New(host, port string, login loginService, user userService, jwtService jwtService, tokenTTL time.Duration) *handler {
	return &handler{
		host:     host,
		port:     port,
		auth:     login,
		user:     user,
		jwt:      jwtService,
		tokenTTL: tokenTTL,
	}
}

func (h *handler) RegisterRoutes(e *echo.Echo) {
	e.GET("/api/v1/login", h.login)
	e.GET("/api/v1/users", h.users, h.jwt.Authorization)
	e.GET(callbackPath, h.callback)
}

func (h *handler) login(c echo.Context) error {
	path := h.host + ":" + h.port + callbackPath
	return c.Redirect(http.StatusTemporaryRedirect, h.auth.RedirectURL(path, c.RealIP()))
}

func (h *handler) users(c echo.Context) error {
	var users usersResponse
	all, err := h.user.All(c.Request().Context())
	if err != nil {
		return err
	}
	users.Users = make([]userResponse, 0, len(all))
	for i := range all {
		u := &all[i]
		users.Users = append(users.Users, userResponse{
			ID:       u.ID,
			Username: u.Username,
			Avatar:   u.Avatar,
		})
	}

	return c.JSON(http.StatusOK, users)
}

func (h *handler) callback(c echo.Context) error {
	ctx := c.Request().Context()
	userID, username, avatar, err := h.auth.GetDiscordInfo(ctx, c.FormValue("code"), c.FormValue("state"), c.RealIP())
	switch {
	case errors.Is(err, auth.ErrUnknownUser):
		return c.String(http.StatusNotFound, "Unknown user, ask andrei.khodko@gmail.com to add")
	case err != nil:
		return err
	}

	token, err := h.jwt.Generate(userID)
	if err != nil {
		return err
	}

	avatar = discordAvatarURL + userID + "/" + avatar
	if err := h.user.Upsert(ctx, userID, username, avatar); err != nil {
		return err
	}

	resp := loginResponse{
		Token:      token,
		Username:   username,
		Avatar:     avatar,
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

type userResponse struct {
	ID       string `json:"id,omitempty"`
	Username string `json:"username,omitempty"`
	Avatar   string `json:"avatar,omitempty"`
}

type usersResponse struct {
	Users []userResponse `json:"users,omitempty"`
}
