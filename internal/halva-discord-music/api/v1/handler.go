package apiv1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/HalvaPovidlo/halva-services/internal/halva-discord-music/api/v1/socket"
	pds "github.com/HalvaPovidlo/halva-services/internal/halva-discord-music/discord"
	"github.com/HalvaPovidlo/halva-services/internal/halva-discord-music/music/player"
	"github.com/HalvaPovidlo/halva-services/internal/halva-discord-music/music/search"
	"github.com/HalvaPovidlo/halva-services/internal/pkg/song"
	"github.com/HalvaPovidlo/halva-services/pkg/contexts"
)

type commandType string

const (
	CommandPlay     commandType = "play"
	CommandSkip     commandType = "skip"
	CommandLoop     commandType = "loop"
	CommandLoopOff  commandType = "loop_off"
	CommandRadio    commandType = "radio"
	CommandRadioOff commandType = "radio_off"
)

type jwtService interface {
	Authorization(next echo.HandlerFunc) echo.HandlerFunc
	ExtractUserID(c echo.Context) (string, error)
}

type SocketManager interface {
	Open(c echo.Context, userID discord.UserID) error
	Write(data []byte, userID discord.UserID, id uuid.UUID) error
	WriteAll(data []byte) error
	ReadChan() <-chan socket.Data
}

type Player interface {
	Input() chan<- *player.Command
	Status() <-chan player.State
	SubscribeOnErrors(h player.ErrorHandler)
}

type Command struct {
	Type    commandType      `json:"type"`
	Query   string           `json:"query,omitempty"`
	Service song.ServiceType `json:"service,omitempty"`
	TraceID string           `json:"trace_id,omitempty"`
}

type outputMessage struct {
	Error        bool   `json:"is_error"`
	ErrorMessage string `json:"error_message"`
	player.State
}

type handler struct {
	player Player
	socket SocketManager
	jwt    jwtService

	host string
	port string
	web  string
}

func New(ctx context.Context, player Player, manager SocketManager, jwt jwtService) *handler {
	h := &handler{
		player: player,
		socket: manager,
		jwt:    jwt,
	}
	h.player.SubscribeOnErrors(h.playerErrorHandler)
	go h.readStatus(ctx)
	go h.readSocket(ctx)
	return h
}

func (h *handler) RegisterRoutes(e *echo.Echo) {
	e.GET("/api/v1/status", h.open)
	e.GET("/api/v1/control", h.open, h.jwt.Authorization)
}

func (h *handler) open(c echo.Context) error {
	id, _ := h.jwt.ExtractUserID(c)
	if id == "" {
		return h.socket.Open(c, 0)
	}

	parsed, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Errorf("parse userID(%s): %+w", id, err).Error())
	}
	userID := discord.UserID(parsed)

	return h.socket.Open(c, userID)
}

func (h *handler) readSocket(ctx context.Context) {
	input := h.socket.ReadChan()
	logger := contexts.GetLogger(ctx)
	for {
		select {
		case data := <-input:
			if data.UserID == discord.NullUserID {
				continue
			}
			var cmd Command
			if err := json.Unmarshal(data.Bytes, &cmd); err != nil {
				logger.Error("failed to unmarshall data from socket", zap.Error(err))
				continue
			}

			ctx := contexts.WithCommandValues(ctx, string(cmd.Type), logger, cmd.TraceID)
			logger := contexts.GetLogger(ctx).With(zap.Stringer("userID", data.UserID), zap.Stringer("socketID", data.SocketID))

			logger.Info("process command from socket")
			if err := h.processCommand(ctx, &cmd, data.UserID); err != nil {
				logger.Error("failed to process command from socket", zap.Error(err))
				if err := h.writeError(err, data.UserID, data.SocketID); err != nil {
					logger.Error("failed to write error message to socket", zap.Error(err))
				}
			}

		case <-ctx.Done():
			return
		}
	}
}

func (h *handler) processCommand(ctx context.Context, cmd *Command, userID discord.UserID) error {
	voiceState, err := pds.State.VoiceState(pds.HalvaGuildID, userID)
	if err != nil {
		return fmt.Errorf("get user voice state: %+w", err)
	}

	command := &player.Command{
		UserID:         userID,
		VoiceChannelID: voiceState.ChannelID,
		TraceID:        contexts.GetTraceID(ctx),
	}

	playerInput := h.player.Input()
	switch cmd.Type {
	case CommandPlay:
		command.Type = player.CommandEnqueue
		command.SearchRequest = &search.Request{
			Text:    cmd.Query,
			UserID:  userID.String(),
			Service: cmd.Service,
		}
	case CommandSkip:
		command.Type = player.CommandSkip
	case CommandLoop:
		command.Type = player.CommandLoop
	case CommandLoopOff:
		command.Type = player.CommandLoopOff
	case CommandRadio:
		command.Type = player.CommandRadio
	case CommandRadioOff:
		command.Type = player.CommandRadioOff
	default:
		return fmt.Errorf("unknown command: %s", command.Type)
	}
	go func() { playerInput <- command }()
	return nil
}

func (h *handler) readStatus(ctx context.Context) {
	input := h.player.Status()
	logger := contexts.GetLogger(ctx)
	for {
		select {
		case state := <-input:
			if len(state.Queue) > 0 {
				state.Queue = state.Queue[1:]
			}
			err := h.writeStatus(state)
			if err != nil {
				logger.Error("write status to sockets", zap.Error(err))
			}
		case <-ctx.Done():
			return
		}
	}
}

func (h *handler) writeStatus(state player.State) error {
	bytes, err := json.Marshal(outputMessage{
		State: state,
	})
	if err != nil {
		return fmt.Errorf("marshal state: %+w", err)
	}

	if err := h.socket.WriteAll(bytes); err != nil {
		return fmt.Errorf("write data to all: %+w", err)
	}

	return nil
}

func (h *handler) writeError(err error, userID discord.UserID, socketID uuid.UUID) error {
	if err == nil {
		return nil
	}

	bytes, err := json.Marshal(outputMessage{
		Error:        true,
		ErrorMessage: err.Error(),
	})
	if err != nil {
		return fmt.Errorf("marshal state: %+w", err)
	}

	if err := h.socket.Write(bytes, userID, socketID); err != nil {
		return fmt.Errorf("write data to all: %+w", err)
	}

	return nil
}

func (h *handler) playerErrorHandler(err error) {
	if err == nil {
		return
	}
	bytes, err := json.Marshal(outputMessage{
		Error:        true,
		ErrorMessage: err.Error(),
	})
	if err != nil {
		return
	}

	_ = h.socket.WriteAll(bytes)
}
