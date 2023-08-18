package player

import (
	"context"

	"github.com/diamondburned/arikawa/v3/discord"
	"go.uber.org/zap"

	"github.com/HalvaPovidlo/halva-services/internal/halva-discord-music/music/download"
	"github.com/HalvaPovidlo/halva-services/internal/halva-discord-music/music/search"
	psong "github.com/HalvaPovidlo/halva-services/internal/pkg/song"
	"github.com/HalvaPovidlo/halva-services/pkg/contexts"
)

const (
	commandPlay           = "play"
	commandSkip           = "skip"
	commandDisconnect     = "disconnect"
	commandDeleteSong     = "delete"
	commandSendState      = "state"
	commandDisconnectIdle = "disconnect_idle"
)

type Command struct {
	typ            string
	userID         discord.UserID
	voiceChannelID discord.ChannelID

	downloadRequest *download.Request
	searchRequest   *search.Request

	traceID string
}

func Play(query string, service psong.ServiceType, userID discord.UserID, voiceID discord.ChannelID, traceID string) *Command {
	return &Command{
		typ:            commandPlay,
		userID:         userID,
		voiceChannelID: voiceID,
		searchRequest: &search.Request{
			Text:    query,
			UserID:  userID.String(),
			Service: service,
		},
		traceID: traceID,
	}
}

func Skip(userID discord.UserID, voiceID discord.ChannelID, traceID string) *Command {
	return &Command{typ: commandSkip, userID: userID, voiceChannelID: voiceID, traceID: traceID}
}

func Disconnect(userID discord.UserID, voiceID discord.ChannelID, traceID string) *Command {
	return &Command{typ: commandDisconnect, userID: userID, voiceChannelID: voiceID, traceID: traceID}
}

func (c *Command) contextLogger(ctx context.Context) (context.Context, *zap.Logger) {
	fields := []zap.Field{zap.String("command", c.typ)}

	if c.userID != discord.NullUserID {
		fields = append(fields, zap.Stringer("userID", c.userID))
	}
	if c.voiceChannelID != discord.NullChannelID {
		fields = append(fields, zap.Stringer("voiceID", c.voiceChannelID))
	}
	if c.downloadRequest != nil {
		fields = append(fields, zap.String("songPath", c.downloadRequest.Source))
	}
	if c.searchRequest != nil {
		fields = append(fields,
			zap.String("search.Request", string(c.searchRequest.Service)+"_"+c.searchRequest.Text),
		)
	}

	logger := contexts.GetLogger(ctx).With(fields...)
	nctx := contexts.WithValues(ctx, logger, c.traceID)
	return nctx, contexts.GetLogger(nctx)
}
