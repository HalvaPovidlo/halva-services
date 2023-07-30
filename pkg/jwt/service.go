package jwt

import (
	"time"

	"github.com/golang-jwt/jwt/v4"
	echojwt "github.com/labstack/echo-jwt/v4"
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
)

const (
	contextKey  = "jwt_key"
	userIDClaim = "userID_jwt"
)

var TokenTTL = time.Second * 10

type Claims struct {
	UserID string `json:"userID"`
	jwt.RegisteredClaims
}

type service struct {
	secret        []byte
	signingMethod jwt.SigningMethod
}

func New(secret string) *service {
	return &service{
		secret:        []byte(secret),
		signingMethod: jwt.SigningMethodHS256,
	}
}

func (s *service) Generate(userID string) (string, error) {
	token := jwt.NewWithClaims(s.signingMethod, &Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(TokenTTL)),
		},
	})

	hash, err := token.SignedString(s.secret)
	return hash, errors.Wrap(err, "sign jwt with secret")
}

func (s *service) Authorization(next echo.HandlerFunc) echo.HandlerFunc {
	return s.tokenExtractor()(func(c echo.Context) error {
		token, ok := c.Get(contextKey).(*jwt.Token)
		if !ok {
			return errors.New("JWT token missing or invalid")
		}
		claims, ok := token.Claims.(*Claims)
		if !ok {
			return errors.New("failed to cast claims")
		}

		c.Set(userIDClaim, claims.UserID)
		return next(c)
	})
}

func (s *service) ExtractUserID(c echo.Context) (string, error) {
	if v, ok := c.Get(userIDClaim).(string); ok {
		return v, nil
	}
	return "", errors.New("bad userID claim")
}

func (s *service) tokenExtractor() echo.MiddlewareFunc {
	return echojwt.WithConfig(echojwt.Config{
		ContextKey:    contextKey,
		SigningKey:    s.secret,
		SigningMethod: s.GetSigningMethod().Alg(),
		TokenLookup:   "header:Authorization:Bearer ,query:token:",
		NewClaimsFunc: func(c echo.Context) jwt.Claims {
			return &Claims{}
		},
	})
}

func (s *service) GetSigningMethod() jwt.SigningMethod {
	return s.signingMethod
}
