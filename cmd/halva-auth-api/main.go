package main

import (
	"time"

	"github.com/HalvaPovidlo/halva-services/cmd/halva-auth-api/config"
	apiv1 "github.com/HalvaPovidlo/halva-services/internal/halva-auth-api/api/v1"
	"github.com/HalvaPovidlo/halva-services/internal/halva-auth-api/auth"
	"github.com/HalvaPovidlo/halva-services/pkg/jwt"
)

var ttl = time.Hour * 48

const configPath = "cmd/halva-auth-api/config/secret.yaml"

func main() {
	cfg, err := config.InitConfig(configPath, "")
	if err != nil {
		panic(err)
	}

	jwtService := jwt.New(cfg.General.Secret, ttl)
	authService := auth.New(cfg.Login)
	handler := apiv1.New(cfg.General.Host, cfg.General.Port, authService, jwtService, ttl)
	handler.Run()
}
