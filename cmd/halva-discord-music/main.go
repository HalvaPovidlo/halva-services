package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	pcache "github.com/patrickmn/go-cache"
	"go.uber.org/zap"

	"github.com/HalvaPovidlo/halva-services/cmd/halva-discord-music/config"
	apiv1 "github.com/HalvaPovidlo/halva-services/internal/halva-discord-music/api/v1"
	"github.com/HalvaPovidlo/halva-services/internal/halva-discord-music/api/v1/socket"
	"github.com/HalvaPovidlo/halva-services/internal/halva-discord-music/discord"
	"github.com/HalvaPovidlo/halva-services/internal/halva-discord-music/music/download"
	"github.com/HalvaPovidlo/halva-services/internal/halva-discord-music/music/firestore"
	"github.com/HalvaPovidlo/halva-services/internal/halva-discord-music/music/player"
	"github.com/HalvaPovidlo/halva-services/internal/halva-discord-music/music/player/playlist"
	"github.com/HalvaPovidlo/halva-services/internal/halva-discord-music/music/search"
	"github.com/HalvaPovidlo/halva-services/pkg/contexts"
	"github.com/HalvaPovidlo/halva-services/pkg/echos"
	fire "github.com/HalvaPovidlo/halva-services/pkg/firestore"
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
	ctx := contexts.WithLogger(context.Background(), logger)

	discordClient := discord.NewClient(cfg.Discord, logger, cfg.General.Debug)

	fireClient, err := fire.New(ctx, "halvabot-firebase.json")
	if err != nil {
		logger.Fatal("failed to init firestore client", zap.Error(err))
	}

	fireStorage := firestore.New(firestore.NewStorage(fireClient), firestore.NewCache(pcache.NoExpiration, pcache.NoExpiration))
	if err := fireStorage.FillCache(ctx); err != nil {
		logger.Fatal("fill firestore cache", zap.Error(err))
	}

	searcher, err := search.New(ctx, "halvabot-google.json", fireStorage)
	if err != nil {
		logger.Fatal("failed to init searcher", zap.Error(err))
	}

	downloader, err := download.New("songs")
	if err != nil {
		logger.Fatal("failed to init downloader", zap.Error(err))
	}

	musicPlayer := player.New(ctx, playlist.New(searcher), downloader, time.Duration(cfg.General.StateTicks)*time.Millisecond)

	discordHandler := apiv1.NewDiscord(discordClient, musicPlayer, searcher)
	discordHandler.RegisterRoutes()

	handler := apiv1.New(ctx, discordClient, searcher, musicPlayer, socket.NewManager(ctx), jwt.New(cfg.General.Secret))

	echoServer := echos.New()
	echoServer.RegisterHandlers(handler)
	echoServer.Run(cfg.General.Port, logger)

	if err := discordClient.Connect(ctx); err != nil {
		logger.Fatal("failed to discord connect: ", zap.Error(err))
		return
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	logger.Info("application started")

	<-stop
	logger.Info("signal received, stopping gracefully")
	signal.Stop(stop)
	close(stop)

	if err := echoServer.Shutdown(ctx); err != nil {
		logger.Error("failed echo server shutdown", zap.Error(err))
	}
	discordClient.Close()
	logger.Info("stopped")
}
