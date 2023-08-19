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
	"github.com/pkg/errors"
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
	commandPlay       commandType = "play"
	commandSkip       commandType = "skip"
	commandLoop       commandType = "loop"
	commandLoopOff    commandType = "loop_off"
	commandRadio      commandType = "radio"
	commandRadioOff   commandType = "radio_off"
	commandShuffle    commandType = "shuffle"
	commandShuffleOff commandType = "shuffle_off"
	commandDisconnect commandType = "disconnect"
)

type discordClient interface {
	VoiceState(discord.GuildID, discord.UserID) (*discord.VoiceState, error)
}

type jwtService interface {
	Authorization(next echo.HandlerFunc) echo.HandlerFunc
	ExtractUserID(c echo.Context) (string, error)
}

type socketManager interface {
	Open(c echo.Context, userID discord.UserID) error
	Write(data []byte, userID discord.UserID, id uuid.UUID) error
	WriteAll(data []byte) error
	ReadChan() <-chan socket.Data
}

type playerService interface {
	Play(item *song.Item, voiceID discord.ChannelID, traceID string)
	Skip(voiceID discord.ChannelID, traceID string)
	Disconnect(voiceID discord.ChannelID, traceID string)

	Loop(state bool)
	LoopToggle()
	Radio(state bool, voiceID discord.ChannelID, traceID string)
	RadioToggle(voiceID discord.ChannelID, traceID string)
	Shuffle(state bool)
	ShuffleToggle()

	SubscribeOnErrors(h player.ErrorHandler)
	SubscribeOnStates(h player.StateHandler)
}

type searcher interface {
	Search(ctx context.Context, request *search.Request) (*song.Item, error)
}

type command struct {
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
	player   playerService
	client   discordClient
	socket   socketManager
	searcher searcher
	jwt      jwtService

	host string
	port string
	web  string
}

func New(ctx context.Context, client discordClient, searcher searcher, player playerService, manager socketManager, jwt jwtService) *handler {
	h := &handler{
		player:   player,
		client:   client,
		socket:   manager,
		searcher: searcher,
		jwt:      jwt,
	}
	h.player.SubscribeOnErrors(h.playerErrorHandler)
	h.player.SubscribeOnStates(h.playerStateHandler)
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
			var cmd command
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

func (h *handler) processCommand(ctx context.Context, cmd *command, userID discord.UserID) error {
	voiceState, err := h.client.VoiceState(pds.HalvaGuildID, userID)
	if err != nil {
		return errors.Wrap(err, "get voice state")
	}

	switch cmd.Type {
	case commandPlay:
		s, err := h.searcher.Search(ctx, &search.Request{
			Text:    cmd.Query,
			UserID:  userID,
			Service: cmd.Service,
		})
		if err != nil {
			return errors.Wrap(err, "search song")
		}
		h.player.Play(s, voiceState.ChannelID, contexts.GetTraceID(ctx))
	case commandSkip:
		h.player.Skip(voiceState.ChannelID, contexts.GetTraceID(ctx))
	case commandLoop:
		h.player.Loop(true)
	case commandLoopOff:
		h.player.Loop(false)
	case commandRadio:
		h.player.Radio(true, voiceState.ChannelID, contexts.GetTraceID(ctx))
	case commandRadioOff:
		h.player.Radio(false, voiceState.ChannelID, contexts.GetTraceID(ctx))
	case commandShuffle:
		h.player.Shuffle(true)
	case commandShuffleOff:
		h.player.Shuffle(false)
	case commandDisconnect:
		h.player.Disconnect(voiceState.ChannelID, contexts.GetTraceID(ctx))
	default:
		return errors.New("unknown command")
	}
	return nil
}

func (h *handler) writeStatus(state player.State) error {
	bytes, err := json.Marshal(outputMessage{
		State: state,
	})
	if err != nil {
		return errors.Wrap(err, "marshal state")
	}

	if err := h.socket.WriteAll(bytes); err != nil {
		return errors.Wrap(err, "write data to all")
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

func (h *handler) playerStateHandler(state player.State) {
	h.playerErrorHandler(h.writeStatus(state))
}
