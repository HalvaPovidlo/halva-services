package socket

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

var ErrNoSuchSocket = fmt.Errorf("socket does not exists")

type manager struct {
	ctx     context.Context
	mx      *sync.RWMutex
	sockets map[uuid.UUID]*socket
	read    chan []byte
}

func NewManager(ctx context.Context) *manager {
	return &manager{
		ctx:     ctx,
		mx:      &sync.RWMutex{},
		sockets: make(map[uuid.UUID]*socket),
		read:    make(chan []byte),
	}
}

func (m *manager) readSocket(ctx context.Context, socket *socket) {
	defer func() {
		m.mx.Lock()
		delete(m.sockets, socket.ID())
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
			m.read <- data
		}
	}
}

func (m *manager) Open(c echo.Context) error {
	socket, err := NewSocket(m.ctx, c)
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Errorf("start new socket: %w", err).Error())
	}

	m.mx.Lock()
	m.sockets[socket.ID()] = socket
	m.mx.Unlock()
	m.readSocket(m.ctx, socket)

	return c.String(http.StatusOK, "socket successfully closed")
}

func (m *manager) Write(data []byte, id uuid.UUID) error {
	m.mx.RLock()
	socket, ok := m.sockets[id]
	m.mx.RUnlock()
	if !ok {
		return ErrNoSuchSocket
	}

	if err := socket.Write(data); err != nil {
		return fmt.Errorf("write to the socket: %w", err)
	}
	return nil
}

func (m *manager) WriteAll(data []byte) error {
	m.mx.RLock()
	sockets := make([]*socket, 0, len(m.sockets))
	for _, v := range m.sockets {
		sockets = append(sockets, v)
	}
	m.mx.RUnlock()

	for i := range sockets {
		if err := sockets[i].Write(data); err != nil {
			return fmt.Errorf("write to the socket: %w", err)
		}
	}
	return nil
}

func (m *manager) ReadChan() <-chan []byte {
	return m.read
}
