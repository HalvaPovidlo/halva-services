package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/patrickmn/go-cache"
	"go.uber.org/zap"

	"github.com/HalvaPovidlo/halva-services/cmd/halva-auth-api/config"
	apiv1 "github.com/HalvaPovidlo/halva-services/internal/halva-auth-api/api/v1"
	"github.com/HalvaPovidlo/halva-services/internal/halva-auth-api/auth"
	"github.com/HalvaPovidlo/halva-services/internal/halva-auth-api/user"
	"github.com/HalvaPovidlo/halva-services/pkg/echos"
	"github.com/HalvaPovidlo/halva-services/pkg/firestore"
	"github.com/HalvaPovidlo/halva-services/pkg/jwt"
	"github.com/HalvaPovidlo/halva-services/pkg/log"
)

const configPathEnv = "CONFIG_PATH"

func main() {
	cfg, err := config.InitConfig(configPathEnv, "")
	if err != nil {
		panic(err)
	}
	logger := log.NewLogger(cfg.General.Debug)

	fireClient, err := firestore.New(context.Background(), "halvabot-firebase.json")
	if err != nil {
		logger.Fatal("failed to init firestore client", zap.Error(err))
	}

	userService := user.New(user.NewCache(cache.NoExpiration, cache.NoExpiration), user.NewStorage(fireClient))
	err = userService.FillCache(context.Background())
	if err != nil {
		logger.Fatal("failed to fill user service cache", zap.Error(err))
	}

	jwtService := jwt.New(cfg.General.Secret)
	authService := auth.New(cfg.Login)
	handler := apiv1.New(cfg.General.Host, cfg.General.Port, cfg.General.Web, authService, userService, jwtService)

	echoServer := echos.New()
	echoServer.RegisterHandlers(handler)
	echoServer.Run(cfg.General.Port, logger)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	logger.Info("application started")

	<-stop
	logger.Info("signal received, stopping gracefully")
	signal.Stop(stop)
	close(stop)

	if err := echoServer.Shutdown(context.Background()); err != nil {
		logger.Error("failed echo server shutdown", zap.Error(err))
	}
	logger.Info("stopped")
}
