package apiv1

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/HalvaPovidlo/halva-services/internal/halva-auth-api/auth"
	"github.com/HalvaPovidlo/halva-services/internal/pkg/user"
	"github.com/HalvaPovidlo/halva-services/pkg/contexts"
	"github.com/HalvaPovidlo/halva-services/pkg/jwt"
)

const (
	callbackPath     = "/auth/callback"
	discordAvatarURL = "https://cdn.discordapp.com/avatars/"
)

type jwtService interface {
	Generate(userID string) (string, error)
	Authorization(next echo.HandlerFunc) echo.HandlerFunc
	ExtractUserID(c echo.Context) (string, error)
}

type loginService interface {
	RedirectURL(redirectURL, key string) string
	GetDiscordInfo(ctx context.Context, authCode, reqState, key string) (string, string, string, error)
	GenerateRefreshToken(userID string) string
	ValidateRefreshToken(userID, token string) (string, error)
	ExpireRefreshToken(token string)
	ExpireAllRefreshTokens(userID string)
}

type userService interface {
	Upsert(ctx context.Context, id, username, avatar string) error
	All(ctx context.Context) (user.Items, error)
}

type handler struct {
	host string
	port string
	web  string
	auth loginService
	jwt  jwtService
	user userService
}

func New(host, port, web string, login loginService, user userService, jwtService jwtService) *handler {
	return &handler{
		host: host,
		port: port,
		web:  web,
		auth: login,
		user: user,
		jwt:  jwtService,
	}
}

func (h *handler) RegisterRoutes(e *echo.Echo) {
	e.GET("/api/v1/login", h.login)
	e.POST("/api/v1/refresh", h.refresh, h.jwt.Authorization)
	e.POST("/api/v1/logout", h.logout, h.jwt.Authorization)
	e.GET("/api/v1/users", h.users, h.jwt.Authorization)
	e.GET(callbackPath, h.callback)
}

func (h *handler) login(c echo.Context) error {
	path := h.host + ":" + h.port + callbackPath
	return c.Redirect(http.StatusTemporaryRedirect, h.auth.RedirectURL(path, c.RealIP()))
}

func (h *handler) logout(c echo.Context) error {
	userID, err := h.jwt.ExtractUserID(c)
	if err != nil {
		return err
	}

	refresh := c.QueryParam("refresh")
	if refresh == "" {
		return c.String(http.StatusBadRequest, "Refresh token is empty")
	}

	all := c.QueryParam("all")
	if all == "true" {
		h.auth.ExpireAllRefreshTokens(userID)
	} else {
		h.auth.ExpireRefreshToken(refresh)
	}

	return c.String(http.StatusOK, "You were successfully logged out")
}

func (h *handler) refresh(c echo.Context) error {
	userID, err := h.jwt.ExtractUserID(c)
	if err != nil {
		return err
	}

	refresh := c.QueryParam("refresh")
	if refresh == "" {
		return c.String(http.StatusBadRequest, "Refresh token is empty")
	}

	newToken, err := h.auth.ValidateRefreshToken(userID, refresh)
	switch {
	case errors.Is(err, auth.ErrInvalidToken):
		return c.String(http.StatusUnprocessableEntity, "Invalid refresh token")
	case err != nil:
		return err
	}

	accessToken, err := h.jwt.Generate(userID)
	if err != nil {
		return err
	}

	resp := loginResponse{
		Token:        accessToken,
		Expiration:   time.Now().Add(jwt.TokenTTL),
		RefreshToken: newToken,
	}
	return c.JSON(http.StatusOK, resp)
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
		contexts.GetLogger(ctx).Warn("Unknown discord user trying to connect!",
			zap.String("id", userID), zap.String("username", username))
		return c.String(http.StatusNotFound, "Unknown user, ask andrei.khodko@gmail.com to add")
	case err != nil:
		return err
	}

	accessToken, err := h.jwt.Generate(userID)
	if err != nil {
		return err
	}

	avatar = discordAvatarURL + userID + "/" + avatar
	if err := h.user.Upsert(ctx, userID, username, avatar); err != nil {
		return err
	}

	resp := loginResponse{
		Token:        accessToken,
		Username:     username,
		Avatar:       avatar,
		Expiration:   time.Now().Add(jwt.TokenTTL),
		RefreshToken: h.auth.GenerateRefreshToken(userID),
	}
	return c.Redirect(http.StatusPermanentRedirect, fmt.Sprintf("%s:%s/%s", h.host, h.web, resp.query()))
}

type loginResponse struct {
	Token        string    `json:"token"`
	Expiration   time.Time `json:"expiration"`
	RefreshToken string    `json:"refresh_token"`
	Username     string    `json:"username,omitempty"`
	Avatar       string    `json:"avatar,omitempty"`
}

func (r *loginResponse) query() string {
	return fmt.Sprintf("?token=%s&username=%s&avatar=%s&refresh_token=%s&expiration=%s", r.Token, r.Username, r.Avatar, r.RefreshToken, r.Expiration.String())
}

type userResponse struct {
	ID       string `json:"id,omitempty"`
	Username string `json:"username,omitempty"`
	Avatar   string `json:"avatar,omitempty"`
}

type usersResponse struct {
	Users []userResponse `json:"users,omitempty"`
}
