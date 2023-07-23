package player

import (
	"context"
	"github.com/diamondburned/arikawa/v3/discord"
	"go.uber.org/zap"

	"github.com/HalvaPovidlo/halva-services/internal/halva-discord-music/music/download"
	"github.com/HalvaPovidlo/halva-services/internal/halva-discord-music/music/search"
	"github.com/HalvaPovidlo/halva-services/pkg/contexts"
)

const (
	CommandSkip       = "skip"
	CommandEnqueue    = "enqueue"
	CommandLoop       = "loop"
	CommandLoopOff    = "loop_off"
	CommandRadio      = "radio"
	CommandRadioOff   = "radio_off"
	CommandShuffle    = "shuffle"
	CommandShuffleOff = "shuffle_off"
	CommandDisconnect = "disconnect"

	commandPlay           = "play"
	commandRemove         = "remove"
	commandDeleteSong     = "delete"
	commandSendState      = "state"
	commandDisconnectIdle = "disconnect_idle"
)

type Command struct {
	Type           string
	UserID         discord.UserID
	VoiceChannelID discord.ChannelID

	downloadRequest *download.Request
	SearchRequest   *search.Request

	TraceID string
}

func (c *Command) ContextLogger(ctx context.Context) (context.Context, *zap.Logger) {
	fields := []zap.Field{
		zap.String("command", c.Type),
		zap.Stringer("voiceID", c.VoiceChannelID),
	}
	if c.UserID != discord.NullUserID {
		fields = append(fields, zap.Stringer("userID", c.UserID))
	}
	if c.VoiceChannelID != discord.NullChannelID {
		fields = append(fields, zap.Stringer("voiceID", c.VoiceChannelID))
	}
	if c.downloadRequest != nil {
		fields = append(fields, zap.String("songPath", c.downloadRequest.Source))
	}
	if c.SearchRequest != nil {
		fields = append(fields,
			zap.String("search.Request", string(c.SearchRequest.Service)+"_"+c.SearchRequest.Text),
		)
	}
	logger := contexts.GetLogger(ctx).With(fields...)
	nctx := contexts.WithValues(ctx, logger, c.TraceID)
	return nctx, contexts.GetLogger(nctx)
}
