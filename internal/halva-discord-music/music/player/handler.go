package player

import (
	"context"

	"github.com/diamondburned/arikawa/v3/discord"
	"go.uber.org/zap"

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

type command struct {
	typ            string
	voiceChannelID discord.ChannelID
	source         string

	traceID string
}

func (c *command) contextLogger(ctx context.Context) (context.Context, *zap.Logger) {
	fields := []zap.Field{zap.String("command", c.typ)}

	if c.voiceChannelID != discord.NullChannelID {
		fields = append(fields, zap.Stringer("voiceID", c.voiceChannelID))
	}

	logger := contexts.GetLogger(ctx).With(fields...)
	nctx := contexts.WithValues(ctx, logger, c.traceID)
	return nctx, contexts.GetLogger(nctx)
}

func (s *service) Play(item *psong.Item, voiceID discord.ChannelID, traceID string) {
	s.playlist.Add(item)
	s.commands <- &command{typ: commandPlay, voiceChannelID: voiceID, traceID: traceID}
}

func (s *service) Skip(voiceID discord.ChannelID, traceID string) {
	s.commands <- &command{typ: commandSkip, voiceChannelID: voiceID, traceID: traceID}
}

func (s *service) Disconnect(voiceID discord.ChannelID, traceID string) {
	s.commands <- &command{typ: commandDisconnect, voiceChannelID: voiceID, traceID: traceID}
}

func (s *service) Loop(state bool) {
	s.playlist.Loop(state)
}

func (s *service) LoopToggle() {
	s.playlist.LoopToggle()
}

func (s *service) Radio(state bool) {
	s.playlist.Radio(state)
}

func (s *service) RadioToggle() {
	s.playlist.RadioToggle()
}

func (s *service) Shuffle(state bool) {
	s.playlist.Shuffle(state)
}

func (s *service) ShuffleToggle() {
	s.playlist.ShuffleToggle()
}
