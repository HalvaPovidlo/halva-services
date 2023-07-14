package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"github.com/HalvaPovidlo/halva-services/cmd/halva-discord-music/config"
	apiv1 "github.com/HalvaPovidlo/halva-services/internal/halva-discord-music/api/v1"
	"github.com/HalvaPovidlo/halva-services/pkg/contexts"
	"github.com/HalvaPovidlo/halva-services/pkg/echos"
	"github.com/HalvaPovidlo/halva-services/pkg/log"
	"github.com/HalvaPovidlo/halva-services/pkg/socket"
)

const configPath = "cmd/halva-auth-api/config/secret.yaml"

func main() {
	cfg, err := config.InitConfig(configPath, "")
	if err != nil {
		panic(err)
	}
	logger := log.NewLogger(cfg.General.Debug)
	ctx := contexts.WithLogger(context.Background(), logger)

	handler := apiv1.New(socket.NewManager(ctx))

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
