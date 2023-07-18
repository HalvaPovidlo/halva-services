package main

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"

	"github.com/HalvaPovidlo/halva-services/cmd/halva-host/config"
	"github.com/HalvaPovidlo/halva-services/pkg/log"
)

const configPathEnv = "CONFIG_PATH"

func main() {
	cfg, err := config.InitConfig(configPathEnv, "")
	if err != nil {
		panic(err)
	}
	logger := log.NewLogger(cfg.General.Debug)

	e := echo.New()
	e.Use(middleware.CORSWithConfig(middleware.DefaultCORSConfig))
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	e.Static("/", "halva")
	for i := range cfg.General.Endpoints {
		endpoint := "/" + cfg.General.Endpoints[i] + "*"
		e.GET(endpoint, func(c echo.Context) error {
			return c.File(cfg.General.Path)
		})
	}

	logger.Fatal("start server:", zap.Error(e.Start(cfg.General.IP+":"+cfg.General.Port)))
}
