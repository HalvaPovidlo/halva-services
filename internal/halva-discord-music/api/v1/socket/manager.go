package socket

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	psocket "github.com/HalvaPovidlo/halva-services/pkg/socket"
)

var ErrNoSuchSocket = fmt.Errorf("socket does not exists")

type Conn interface {
	Write(data []byte) error
	ReadChan() <-chan []byte
	Kill()
}

type Data struct {
	Bytes    []byte
	UserID   discord.UserID
	SocketID uuid.UUID
}

type manager struct {
	ctx     context.Context
	mx      *sync.RWMutex
	sockets map[string]Conn // userID_socketID -> socket
	read    chan Data
}

func NewManager(ctx context.Context) *manager {
	return &manager{
		ctx:     ctx,
		mx:      &sync.RWMutex{},
		sockets: make(map[string]Conn),
		read:    make(chan Data),
	}
}

func (m *manager) readSocket(ctx context.Context, userID discord.UserID, socketID uuid.UUID, socket Conn) {
	id := key(userID, socketID)
	defer func() {
		m.mx.Lock()
		if s, ok := m.sockets[id]; ok {
			s.Kill()
			delete(m.sockets, id)
		}
		m.mx.Unlock()
	}()

	read := socket.ReadChan()
	for {
		select {
		case <-ctx.Done():
			return
		case data, ok := <-read:
			if !ok {
				return
			}
			m.read <- Data{
				Bytes:    data,
				UserID:   userID,
				SocketID: socketID,
			}
		}
	}
}

func (m *manager) Open(c echo.Context, userID discord.UserID) error {
	socket, err := psocket.NewSocket(m.ctx, c)
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Errorf("start new socket: %+w", err).Error())
	}
	socketID := uuid.New()
	id := key(userID, socketID)

	m.mx.Lock()
	if s, ok := m.sockets[id]; ok {
		s.Kill()
	}
	m.sockets[id] = socket
	m.mx.Unlock()

	m.readSocket(m.ctx, userID, socketID, socket)
	return c.String(http.StatusOK, "socket successfully closed")
}

func (m *manager) Write(data []byte, userID discord.UserID, id uuid.UUID) error {
	m.mx.RLock()
	socket, ok := m.sockets[key(userID, id)]
	m.mx.RUnlock()
	if !ok {
		return ErrNoSuchSocket
	}

	if err := socket.Write(data); err != nil {
		return fmt.Errorf("write to the socket: %+w", err)
	}
	return nil
}

func (m *manager) WriteAll(data []byte) error {
	m.mx.RLock()
	sockets := make([]Conn, 0, len(m.sockets))
	for _, v := range m.sockets {
		sockets = append(sockets, v)
	}
	m.mx.RUnlock()

	for i := range sockets {
		if err := sockets[i].Write(data); err != nil {
			return fmt.Errorf("write to the socket: %+w", err)
		}
	}
	return nil
}

func (m *manager) ReadChan() <-chan Data {
	return m.read
}

func key(userID discord.UserID, socketID uuid.UUID) string {
	return userID.String() + "_" + socketID.String()
}
