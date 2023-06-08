package echos

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/HalvaPovidlo/halva-services/pkg/contexts"
)

type Handler interface {
	RegisterRoutes(e *echo.Echo)
}

type service struct {
	echo *echo.Echo
}

func New() *service {
	e := echo.New()
	e.Use(middleware.CORSWithConfig(middleware.DefaultCORSConfig))
	return &service{
		echo: e,
	}
}

func (s *service) RegisterHandlers(handlers ...Handler) {
	for i := range handlers {
		handlers[i].RegisterRoutes(s.echo)
	}
}

func (s *service) Run(port string, log *zap.Logger) {
	s.echo.Use(loggerMiddleware(log))
	s.echo.Use(recoverMiddleware())

	go func() {
		if err := s.echo.Start(":" + port); err != nil && err != http.ErrServerClosed {
			log.Fatal("shutting down the server", zap.Error(err))
		}
	}()
}

func (s *service) Shutdown(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return s.echo.Shutdown(ctx)
}

func loggerMiddleware(log *zap.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			traceID := c.Request().Header.Get(echo.HeaderXRequestID)
			contexts.SetValuesEcho(c, log, traceID)
			traceID = contexts.GetTraceID(c.Request().Context())
			log.Info("Request", zap.String("trace_id", traceID))

			start := time.Now()
			err := next(c)
			if err != nil {
				c.Error(err)
			}

			req := c.Request()
			res := c.Response()

			fields := []zapcore.Field{
				zap.String("trace_id", traceID),
				zap.String("remote_ip", c.RealIP()),
				zap.String("latency", time.Since(start).String()),
				zap.String("host", req.Host),
				zap.String("request", fmt.Sprintf("%s %s", req.Method, req.RequestURI)),
				zap.Int("status", res.Status),
				zap.Int64("size", res.Size),
				zap.String("user_agent", req.UserAgent()),
			}

			n := res.Status
			switch {
			case n >= 500:
				log.With(zap.Error(err)).Error("Server error", fields...)
			case n >= 400:
				log.With(zap.Error(err)).Warn("Client error", fields...)
			case n >= 300:
				log.Info("Redirection", fields...)
			default:
				log.Info("Success", fields...)
			}

			return nil
		}
	}
}

func recoverMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			logger := contexts.GetLogger(c.Request().Context())
			defer func() {
				if r := recover(); r != nil {
					if r == http.ErrAbortHandler {
						panic(r)
					}
					err, ok := r.(error)
					if !ok {
						err = fmt.Errorf("%v", r)
					}
					logger.Error("Panic during processing request", zap.Error(err))
					c.Error(err)
				}
			}()
			return next(c)
		}
	}
}
