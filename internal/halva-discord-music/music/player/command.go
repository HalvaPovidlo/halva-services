package player

import (
	"context"

	"github.com/diamondburned/arikawa/v3/discord"
	"go.uber.org/zap"

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

type command struct {
	typ            string
	userID         discord.UserID
	voiceChannelID discord.ChannelID
	source         string

	traceID string
}

func play(userID discord.UserID, voiceID discord.ChannelID, traceID string) *command {
	return &command{typ: commandPlay, userID: userID, voiceChannelID: voiceID, traceID: traceID}
}

func skip(userID discord.UserID, voiceID discord.ChannelID, traceID string) *command {
	return &command{typ: commandSkip, userID: userID, voiceChannelID: voiceID, traceID: traceID}
}

func disconnect(userID discord.UserID, voiceID discord.ChannelID, traceID string) *command {
	return &command{typ: commandDisconnect, userID: userID, voiceChannelID: voiceID, traceID: traceID}
}

func (c *command) contextLogger(ctx context.Context) (context.Context, *zap.Logger) {
	fields := []zap.Field{zap.String("command", c.typ)}

	if c.userID != discord.NullUserID {
		fields = append(fields, zap.Stringer("userID", c.userID))
	}
	if c.voiceChannelID != discord.NullChannelID {
		fields = append(fields, zap.Stringer("voiceID", c.voiceChannelID))
	}

	logger := contexts.GetLogger(ctx).With(fields...)
	nctx := contexts.WithValues(ctx, logger, c.traceID)
	return nctx, contexts.GetLogger(nctx)
}
