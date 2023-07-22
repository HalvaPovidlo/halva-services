package player

import (
	"context"
	"strconv"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/HalvaPovidlo/halva-services/internal/halva-discord-music/music/download"
	"github.com/HalvaPovidlo/halva-services/internal/halva-discord-music/music/search"
	psong "github.com/HalvaPovidlo/halva-services/internal/pkg/song"
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

type Command struct {
	t               int
	downloadRequest *download.Request
	searchRequest   *search.Request
	voiceChannel    discord.ChannelID

	traceID string
}

func command(t int, song *search.Request, download *download.Request, voice discord.ChannelID, traceID string) *Command {
	if traceID == "" {
		traceID = uuid.New().String()
	}

	return &Command{
		t:               t,
		searchRequest:   song,
		downloadRequest: download,
		voiceChannel:    voice,
	}
}

func cmdPlay(voiceChannel discord.ChannelID, traceID string) *Command {
	return command(commandPlay, nil, nil, voiceChannel, traceID)
}

func CmdSkip(traceID string) *Command {
	return command(commandSkip, nil, nil, 0, traceID)
}

func CmdEnqueuePlay(query string, service psong.ServiceType, voiceChannel discord.ChannelID, traceID string) *Command {
	return command(commandEnqueue, &search.Request{
		Text:    query,
		Service: service,
	}, nil, voiceChannel, traceID)
}

func CmdEnqueue(query string, service psong.ServiceType, traceID string) *Command {
	return command(commandEnqueue, &search.Request{
		Text:    query,
		Service: service,
	}, nil, discord.NullChannelID, traceID)
}

func CmdLoop(traceID string) *Command {
	return command(commandLoop, nil, nil, discord.NullChannelID, traceID)

}

func CmdLoopOff(traceID string) *Command {
	return command(commandLoopOff, nil, nil, discord.NullChannelID, traceID)

}

func CmdRadio(traceID string) *Command {
	return command(commandRadio, nil, nil, discord.NullChannelID, traceID)

}

func CmdRadioOff(traceID string) *Command {
	return command(commandRadioOff, nil, nil, discord.NullChannelID, traceID)

}

func CmdShuffle(traceID string) *Command {
	return command(commandShuffle, nil, nil, discord.NullChannelID, traceID)
}

func CmdShuffleOff(traceID string) *Command {
	return command(commandShuffleOff, nil, nil, discord.NullChannelID, traceID)
}

func CmdDisconnect(traceID string) *Command {
	return command(commandDisconnect, nil, nil, discord.NullChannelID, traceID)
}

func (c *Command) String() string {
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

func (c *Command) ContextLogger(ctx context.Context) (context.Context, *zap.Logger) {
	fields := []zap.Field{
		zap.Stringer("command", c),
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
	return contexts.WithValues(ctx, logger, c.traceID), logger
}
