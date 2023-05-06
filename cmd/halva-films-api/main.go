package main

import (
	"context"
	"time"

	"github.com/patrickmn/go-cache"

	"github.com/HalvaPovidlo/halva-services/cmd/halva-films-api/config"
	apiv1 "github.com/HalvaPovidlo/halva-services/internal/halva-films-api/api/v1"
	"github.com/HalvaPovidlo/halva-services/internal/halva-films-api/film"
	"github.com/HalvaPovidlo/halva-services/internal/halva-films-api/kinopoisk"
	"github.com/HalvaPovidlo/halva-services/pkg/firestore"
	"github.com/HalvaPovidlo/halva-services/pkg/jwt"
)

var ttl = time.Hour * 48

const configPath = "cmd/halva-films-api/config/secret.yaml"

func main() {
	cfg, err := config.InitConfig(configPath, "")
	if err != nil {
		panic(err)
	}

	fireClient, err := firestore.New(context.Background(), "halvabot-firebase.json")
	if err != nil {
		panic(err)
	}

	filmService := film.New(
		kinopoisk.New(cfg.General.Kinopoisk),
		film.NewCache(cache.NoExpiration, cache.NoExpiration),
		film.NewStorage(fireClient),
	)
	err = filmService.FillCache(context.Background())
	if err != nil {
		panic(err)
	}

	jwtService := jwt.New(cfg.General.Secret, ttl)
	handler := apiv1.New(filmService, jwtService)
	handler.Run(cfg.General.Port)
}
