package socket

import (
	"context"
	"fmt"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/HalvaPovidlo/halva-services/pkg/contexts"
)

type socket struct {
	mx     *sync.Mutex
	conn   *websocket.Conn
	sender string
	read   chan []byte
	cancel context.CancelFunc
}

func NewSocket(ctx context.Context, c echo.Context) (*socket, error) {
	up := &websocket.Upgrader{}
	conn, err := up.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return nil, fmt.Errorf("upgrade echo context to websocket: %+w", err)
	}
	ctx, cancel := context.WithCancel(ctx)

	s := &socket{
		mx:     &sync.Mutex{},
		conn:   conn,
		read:   make(chan []byte),
		sender: conn.RemoteAddr().String(),
		cancel: cancel,
	}
	go s.processRead(ctx)

	return s, nil
}

func (s *socket) processRead(ctx context.Context) {
	defer func() {
		s.conn.Close()
	}()
	defer close(s.read)

	logger := contexts.GetLogger(ctx).With(zap.String("sender", s.sender))
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
					logger.Error("failed to read message from socket", zap.Error(err))
				}
				return
			}

			switch messageType {
			case websocket.TextMessage:
				s.read <- data
			case websocket.CloseMessage:
				return
			}
		}
	}
}

func (s *socket) Write(data []byte) error {
	s.mx.Lock()
	err := s.conn.WriteMessage(websocket.TextMessage, data)
	s.mx.Unlock()
	if err != nil {
		return fmt.Errorf("write message: %+w", err)
	}
	return nil
}

func (s *socket) ReadChan() <-chan []byte {
	return s.read
}

func (s *socket) Kill() {
	s.cancel()
}
