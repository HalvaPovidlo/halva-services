package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

var knownUsers = map[string]struct{}{
	"233195670125805568": {},
	"242030987536629760": {},
	"257456911270674433": {},
	"320309512697413633": {},
	"320310971593916416": {},
	"320311179245256706": {},
	"339482443943772160": {},
	"397466273157480448": {},
	"407858784354959361": {},
	"644504316576530438": {},
}

var (
	ErrBadState     = errors.New("state does not match")
	ErrInvalidToken = errors.New("refresh token is invalid")
	ErrUnknownUser  = errors.New("user unknown")
)

const (
	authURL  = "https://discord.com/api/oauth2/authorize"
	tokenURL = "https://discord.com/api/oauth2/token"
	secret   = "secret"
)

type Config struct {
	ClientID     string   `yaml:"clientID"`
	ClientSecret string   `yaml:"clientSecret"`
	Scopes       []string `yaml:"scopes"`
}

type service struct {
	oauth   *oauth2.Config
	refresh map[string]string // token -> id
	mx      *sync.RWMutex
}

func New(cfg Config) *service {
	return &service{
		oauth: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			Scopes:       cfg.Scopes,
			Endpoint: oauth2.Endpoint{
				AuthURL:   authURL,
				TokenURL:  tokenURL,
				AuthStyle: oauth2.AuthStyleInParams,
			},
		},
		refresh: make(map[string]string, len(knownUsers)*3),
		mx:      &sync.RWMutex{},
	}
}

func (s *service) RedirectURL(redirectURL, key string) string {
	s.oauth.RedirectURL = redirectURL
	return s.oauth.AuthCodeURL(generateState(key))
}

func (s *service) GetDiscordInfo(ctx context.Context, authCode, reqState, key string) (string, string, string, error) {
	state := generateState(key)
	if reqState != state {
		return "", "", "", ErrBadState
	}
	token, err := s.oauth.Exchange(ctx, authCode)
	if err != nil {
		return "", "", "", errors.Wrap(err, "exchange auth code for discord token")
	}

	res, err := s.oauth.Client(context.Background(), token).Get("https://discord.com/api/users/@me")
	if err != nil {
		return "", "", "", errors.Wrap(err, "get my discord info through oauth client")
	}
	if res.StatusCode != http.StatusOK {
		return "", "", "", errors.Wrapf(err, "response status: %s", res.Status)
	}
	defer func() { _ = res.Body.Close() }()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", "", "", errors.Wrap(err, "read response body")
	}

	var discordUser discordOAuthResp
	if err := json.Unmarshal(body, &discordUser); err != nil {
		return "", "", "", errors.Wrap(err, "unmarshal response body")
	}

	if _, ok := knownUsers[discordUser.ID]; !ok {
		return "", "", "", ErrUnknownUser
	}

	return discordUser.ID, discordUser.Username, discordUser.Avatar, nil
}

func (s *service) GenerateRefreshToken(userID string) string {
	token := uuid.New().String()
	s.mx.Lock()
	s.refresh[token] = userID
	s.mx.Unlock()
	return token
}

func (s *service) ExpireRefreshToken(token string) {
	s.mx.Lock()
	delete(s.refresh, token)
	s.mx.Unlock()
}

func (s *service) ExpireAllRefreshTokens(userID string) {
	toDelete := make([]string, 0, len(s.refresh))

	s.mx.RLock()
	for token, id := range s.refresh {
		if id == userID {
			toDelete = append(toDelete, token)
		}
	}
	s.mx.RUnlock()

	for i := range toDelete {
		s.ExpireRefreshToken(toDelete[i])
	}
}

func (s *service) ValidateRefreshToken(userID, token string) (string, error) {
	s.mx.RLock()
	id, ok := s.refresh[token]
	s.mx.RUnlock()
	if !ok || id != userID {
		return "", ErrInvalidToken
	}

	s.ExpireRefreshToken(token)
	return s.GenerateRefreshToken(userID), nil
}

func generateState(key string) string {
	hash := sha256.New()
	hash.Write([]byte(key))
	return base64.StdEncoding.EncodeToString(hash.Sum([]byte(secret)))
}

type discordOAuthResp struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
}
