package player

import (
	"context"
	"strconv"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/HalvaPovidlo/halva-services/internal/halva-discord-music/music/player/search"
	"github.com/HalvaPovidlo/halva-services/pkg/contexts"
)

const (
	commandPlay = iota
	commandSkip
	commandEnqueue
	commandLoop
	commandLoopOff
	commandRadio
	commandRadioOff
	commandShuffle
	commandShuffleOff
	commandDisconnect

	commandDeleteSong
	commandSendState
	commandDisconnectIdle
)

type playerCommand struct {
	t            int
	path         string
	songRequest  *search.SongRequest
	voiceChannel discord.ChannelID

	traceID string
}

func command(t int, song *search.SongRequest, path string, voice discord.ChannelID, traceID string) *playerCommand {
	if traceID == "" {
		traceID = uuid.New().String()
	}

	return &playerCommand{
		t:            t,
		songRequest:  song,
		path:         path,
		voiceChannel: voice,
	}
}

func CmdPlay(voiceChannel discord.ChannelID, traceID string) *playerCommand {
	return command(commandPlay, nil, "", voiceChannel, traceID)
}

func CmdSkip(traceID string) *playerCommand {
	return command(commandSkip, nil, "", 0, traceID)
}

func CmdEnqueue(query string, service search.ServiceType, traceID string) *playerCommand {
	return command(commandEnqueue, &search.SongRequest{
		Text:    query,
		Service: service,
	}, "", 0, traceID)
}

func CmdLoop(traceID string) *playerCommand {
	return command(commandLoop, nil, "", 0, traceID)

}

func CmdLoopOff(traceID string) *playerCommand {
	return command(commandLoopOff, nil, "", 0, traceID)

}

func CmdRadio(traceID string) *playerCommand {
	return command(commandRadio, nil, "", 0, traceID)

}

func CmdRadioOff(traceID string) *playerCommand {
	return command(commandRadioOff, nil, "", 0, traceID)

}

func CmdShuffle(traceID string) *playerCommand {
	return command(commandShuffle, nil, "", 0, traceID)
}

func CmdShuffleOff(traceID string) *playerCommand {
	return command(commandShuffleOff, nil, "", 0, traceID)
}

func CmdDisconnect(traceID string) *playerCommand {
	return command(commandDisconnect, nil, "", 0, traceID)
}

func (c *playerCommand) String() string {
	switch c.t {
	case commandPlay:
		return "play"
	case commandSkip:
		return "skip"
	case commandEnqueue:
		return "enqueue"
	case commandLoop:
		return "loop"
	case commandLoopOff:
		return "loopOff"
	case commandRadio:
		return "radio"
	case commandRadioOff:
		return "radioOff"
	case commandShuffle:
		return "shuffle"
	case commandShuffleOff:
		return "shuffleOff"
	case commandDeleteSong:
		return "deleteSong"
	case commandSendState:
		return "sendState"
	case commandDisconnectIdle:
		return "disconnectIdle"
	case commandDisconnect:
		return "disconnect"
	}
	return "UNKNOWN" + strconv.Itoa(c.t)
}

func (c *playerCommand) ContextLogger(ctx context.Context) (context.Context, *zap.Logger) {
	fields := []zap.Field{
		zap.Stringer("command", c),
	}
	if c.path != "" {
		fields = append(fields, zap.String("songPath", c.path))
	}
	if c.songRequest != nil {
		fields = append(fields,
			zap.String("search.SongRequest", string(c.songRequest.Service)+"_"+c.songRequest.Text),
		)
	}
	logger := contexts.GetLogger(ctx).With(fields...)
	return contexts.WithValues(ctx, logger, c.traceID), logger
}
