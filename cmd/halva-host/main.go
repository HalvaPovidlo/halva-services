package main

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"

	"github.com/HalvaPovidlo/halva-services/pkg/log"
)

func main() {
	logger := log.NewLogger(false)

	e := echo.New()
	e.Use(middleware.CORSWithConfig(middleware.DefaultCORSConfig))
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	e.GET("/*", func(c echo.Context) error {
		return c.File("halva/index.html")
	})

	logger.Fatal("start server:", zap.Error(e.Start(":80")))
}
