package socket

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/HalvaPovidlo/halva-services/pkg/contexts"
)

type socket struct {
	id     uuid.UUID
	conn   *websocket.Conn
	sender string
	read   chan []byte
}

func NewSocket(ctx context.Context, c echo.Context) (*socket, error) {
	up := &websocket.Upgrader{}
	conn, err := up.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return nil, fmt.Errorf("upgrade echo context to websocket: %+w", err)
	}

	s := &socket{
		id:     uuid.New(),
		conn:   conn,
		read:   make(chan []byte),
		sender: conn.RemoteAddr().String(),
	}
	go s.processRead(ctx)

	return s, nil
}

func (s *socket) processRead(ctx context.Context) {
	defer s.conn.Close()
	defer close(s.read)

	logger := contexts.GetLogger(ctx).With(zap.String("id", s.id.String()), zap.String("sender", s.sender))
	for {
		select {
		case <-ctx.Done():
			return
		default:
			messageType, data, err := s.conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
					logger.Info("socket closed")
				} else {
					logger.Error("failed to read message from socket", zap.String("ip", s.sender), zap.Error(err))
				}
				return
			}

			switch messageType {
			case websocket.TextMessage:
				s.read <- append([]byte(s.id.String()), data...)
			case websocket.CloseMessage:
				return
			}
		}
	}
}

func (s *socket) Write(data []byte) error {
	if err := s.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return fmt.Errorf("write message: %+w", err)
	}
	return nil
}

func (s *socket) ReadChan() <-chan []byte {
	return s.read
}

func (s *socket) ID() uuid.UUID {
	return s.id
}
