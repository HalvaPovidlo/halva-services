package apiv1

import (
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type SocketManager interface {
	Write(data []byte, id uuid.UUID) error
	WriteAll(data []byte) error
	ReadChan() <-chan []byte
	Open(c echo.Context) error
}

type handler struct {
	host          string
	port          string
	web           string
	socketManager SocketManager
}

func New(manager SocketManager) *handler {
	return &handler{socketManager: manager}
}

func (h *handler) RegisterRoutes(e *echo.Echo) {
	e.GET("/ws", h.socketManager.Open)
}
